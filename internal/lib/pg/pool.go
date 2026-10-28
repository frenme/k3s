package pg

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type EnvConfig struct {
	URL       string `env:"DATABASE_URL" required:"true"`
	ReadURL   string `env:"DATABASE_READ_URL"`
	DirectURL string `env:"DATABASE_DIRECT_URL"`
	Schema    string `env:"DB_SCHEMA"`
	MaxConns  int32  `env:"DB_MAX_CONNS" default:"10"`
}

type Config struct {
	URL      string
	Schema   string
	MaxConns int32
	Pooled   bool
}

type Pools struct {
	Write      *pgxpool.Pool
	Read       *pgxpool.Pool
	name       string
	collectors []prometheus.Collector
}

func (p *Pools) register(role string, pool *pgxpool.Pool) {
	c := newPoolCollector(p.name, role, pool)
	if err := prometheus.Register(c); err == nil {
		p.collectors = append(p.collectors, c)
	}
}

func (p *Pools) Close() {
	for _, c := range p.collectors {
		prometheus.Unregister(c)
	}
	if p.Read != nil {
		p.Read.Close()
	}
	p.Write.Close()
}

func Open(ctx context.Context, name string, migrations fs.FS, dir string) (*Pools, error) {
	env, err := envconfig.Load[EnvConfig]()
	if err != nil {
		return nil, err
	}
	if env.Schema == "" {
		env.Schema = name
	}
	pooled := env.DirectURL != ""
	migrate := migrations != nil

	if migrate && pooled {
		direct := Config{URL: env.DirectURL, Schema: env.Schema, MaxConns: 2}
		if err := migrateDirect(ctx, direct, migrations, dir); err != nil {
			return nil, err
		}
	}
	write, err := Connect(ctx, Config{URL: env.URL, Schema: env.Schema, MaxConns: env.MaxConns, Pooled: pooled})
	if err != nil {
		return nil, err
	}
	if migrate && !pooled {
		if err := migrateWhenReady(ctx, write, env.Schema, migrations, dir); err != nil {
			write.Close()
			return nil, err
		}
	}

	pools := &Pools{Write: write, name: name}
	pools.register("write", write)
	if env.ReadURL != "" {
		read, err := connectLazy(ctx, Config{URL: env.ReadURL, Schema: env.Schema, MaxConns: env.MaxConns, Pooled: pooled})
		if err != nil {
			pools.Close()
			return nil, err
		}
		pools.Read = read
		pools.register("read", read)
	}
	return pools, nil
}

func migrateDirect(ctx context.Context, c Config, fsys fs.FS, dir string) error {
	pool, err := Connect(ctx, c)
	if err != nil {
		return err
	}
	defer pool.Close()
	return migrateWhenReady(ctx, pool, c.Schema, fsys, dir)
}

func Connect(ctx context.Context, c Config) (*pgxpool.Pool, error) {
	cfg, err := poolConfig(c)
	if err != nil {
		return nil, err
	}
	var pool *pgxpool.Pool
	err = retry(ctx, func() error {
		p, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return err
		}
		if err := p.Ping(ctx); err != nil {
			p.Close()
			return err
		}
		pool = p
		return nil
	}, func(error) bool { return true })
	if err != nil {
		return nil, fmt.Errorf("postgres unavailable: %w", err)
	}
	return pool, nil
}

func connectLazy(ctx context.Context, c Config) (*pgxpool.Pool, error) {
	cfg, err := poolConfig(c)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.ConnectTimeout = 2 * time.Second
	return pgxpool.NewWithConfig(ctx, cfg)
}

func poolConfig(c Config) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(c.URL)
	if err != nil {
		return nil, err
	}
	if c.Pooled {
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheDescribe
	} else if c.Schema != "" {
		cfg.ConnConfig.RuntimeParams["search_path"] = c.Schema
	}
	if c.MaxConns > 0 {
		cfg.MaxConns = c.MaxConns
	}
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	cfg.ConnConfig.Tracer = otelpgx.NewTracer()
	return cfg, nil
}

func retry(ctx context.Context, fn func() error, retryable func(error) bool) error {
	delay := time.Second
	for {
		err := fn()
		if err == nil || !retryable(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(delay):
		}
		delay = min(delay*2, 10*time.Second)
	}
}
