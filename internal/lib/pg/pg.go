package pg

import (
	"context"
	"fmt"
	"hash/fnv"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/egor/k3s-base/internal/lib/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	URL      string
	Schema   string
	MaxConns int32
}

func Connect(ctx context.Context, c Config) (*pgxpool.Pool, error) {
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

	var pool *pgxpool.Pool
	for attempt := 1; attempt <= 30; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, cfg)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				return pool, nil
			}
			pool.Close()
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("postgres unavailable: %w", err)
}

type Pools struct {
	Write *pgxpool.Pool
	Read  *pgxpool.Pool
}

func (p *Pools) Close() {
	if p.Read != nil {
		p.Read.Close()
	}
	p.Write.Close()
}

func Query[T any](p *Pools, fn func(*pgxpool.Pool) (T, error)) (T, error) {
	if p.Read == nil {
		return fn(p.Write)
	}
	v, err := fn(p.Read)
	if err == nil {
		return v, nil
	}
	return fn(p.Write)
}

func Open(ctx context.Context, defaultSchema string, fsys fs.FS, dir string) (*Pools, error) {
	schema := config.String("DB_SCHEMA", defaultSchema)
	maxConns := int32(config.Int("DB_MAX_CONNS", 10))
	write, err := Connect(ctx, Config{
		URL:      config.String("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/app?sslmode=disable"),
		Schema:   schema,
		MaxConns: maxConns,
	})
	if err != nil {
		return nil, err
	}
	if err := Migrate(ctx, write, schema, fsys, dir); err != nil {
		write.Close()
		return nil, err
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

func connectLazy(ctx context.Context, c Config) (*pgxpool.Pool, error) {
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
	cfg.ConnConfig.ConnectTimeout = 2 * time.Second
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second
	return pgxpool.NewWithConfig(ctx, cfg)
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, schema string, fsys fs.FS, dir string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	h := fnv.New64a()
	_, _ = h.Write([]byte(schema))
	lockID := int64(h.Sum64() >> 1)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockID)

	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)", schema).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err := conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
			return err
		}
	}
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		body, err := fs.ReadFile(fsys, dir+"/"+name)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
