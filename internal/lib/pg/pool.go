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

func Open(ctx context.Context, name string, fsys fs.FS, dir string) (*Pools, error) {
	var env EnvConfig
	if err := envconfig.Load(&env); err != nil {
		return nil, err
	}
	if env.Schema == "" {
		env.Schema = name
	}
	pooled := env.DirectURL != ""

	var write *pgxpool.Pool
	var err error
	switch {
	case fsys == nil:
		write, err = Connect(ctx, Config{URL: env.URL, Schema: env.Schema, MaxConns: env.MaxConns, Pooled: pooled})
	case pooled:
		if err = migrateDirect(ctx, Config{URL: env.DirectURL, Schema: env.Schema, MaxConns: 2}, fsys, dir); err == nil {
			write, err = Connect(ctx, Config{URL: env.URL, Schema: env.Schema, MaxConns: env.MaxConns, Pooled: true})
		}
	default:
		write, err = Connect(ctx, Config{URL: env.URL, Schema: env.Schema, MaxConns: env.MaxConns})
		if err == nil {
			if err = migrateWhenReady(ctx, write, env.Schema, fsys, dir); err != nil {
				write.Close()
			}
		}
	}
	if err != nil {
		return nil, err
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
