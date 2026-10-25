package pg

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/egor/k3s-base/internal/lib/config"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	URL      string
	Schema   string
	MaxConns int32
}

func Open(ctx context.Context, defaultSchema string, fsys fs.FS, dir string) (*Pools, error) {
	schema := config.String("DB_SCHEMA", defaultSchema)
	maxConns := config.Int32("DB_MAX_CONNS", 10)

	write, err := Connect(ctx, Config{
		URL:      config.String("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/app?sslmode=disable"),
		Schema:   schema,
		MaxConns: maxConns,
	})
	if err != nil {
		return nil, err
	}
	if fsys != nil {
		if err := migrateWhenReady(ctx, write, schema, fsys, dir); err != nil {
			write.Close()
			return nil, err
		}
	}

	pools := &Pools{Write: write}
	if url := config.String("DATABASE_READ_URL", ""); url != "" {
		read, err := connectLazy(ctx, Config{URL: url, Schema: schema, MaxConns: maxConns})
		if err != nil {
			write.Close()
			return nil, err
		}
		pools.Read = read
	}
	return pools, nil
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
	if c.Schema != "" {
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
