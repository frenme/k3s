package pg

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	insufficientPrivilege = "42501"
	noTxSuffix            = ".notx.sql"
)

func migrateWhenReady(ctx context.Context, pool *pgxpool.Pool, schema string, fsys fs.FS, dir string) error {
	return retry(ctx, func() error {
		return Migrate(ctx, pool, schema, fsys, dir)
	}, func(err error) bool {
		var pgErr *pgconn.PgError
		return errors.As(err, &pgErr) && pgErr.Code == insufficientPrivilege
	})
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, schema string, fsys fs.FS, dir string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	lockID := advisoryLockID(schema)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		return err
	}
	defer func() { _, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockID) }()

	if err := ensureSchema(ctx, conn, schema); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, "ALTER ROLE CURRENT_USER SET search_path = "+pgx.Identifier{schema}.Sanitize()); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}

	names, err := migrationFiles(fsys, dir)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := apply(ctx, conn, fsys, dir, name); err != nil {
			return err
		}
	}
	return nil
}

func advisoryLockID(schema string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(schema))
	return int64(h.Sum64() >> 1)
}

func ensureSchema(ctx context.Context, conn *pgxpool.Conn, schema string) error {
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)", schema).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err := conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	return err
}

func migrationFiles(fsys fs.FS, dir string) ([]string, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func apply(ctx context.Context, conn *pgxpool.Conn, fsys fs.FS, dir, name string) error {
	var applied bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", name).Scan(&applied); err != nil {
		return err
	}
	if applied {
		return nil
	}
	body, err := fs.ReadFile(fsys, dir+"/"+name)
	if err != nil {
		return err
	}
	if strings.HasSuffix(name, noTxSuffix) {
		if _, err := conn.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		_, err := conn.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name)
		return err
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, string(body)); err != nil {
		return fmt.Errorf("migration %s: %w", name, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
