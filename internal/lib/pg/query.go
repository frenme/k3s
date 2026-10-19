package pg

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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

func All[T any](ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) ([]T, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[T])
}

func One[T any](ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) (T, error) {
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		var zero T
		return zero, err
	}
	return pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[T])
}
