package storage

import (
	"context"
	"errors"

	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const orderColumns = `id::text AS id, user_id::text AS user_id, item, amount_cents, created_at`

type Orders struct {
	pools *pg.Pools
}

func NewOrders(pools *pg.Pools) *Orders {
	return &Orders{pools: pools}
}

func (s *Orders) List(ctx context.Context, userID string, limit int) ([]model.Order, error) {
	query := `SELECT ` + orderColumns + ` FROM orders ORDER BY created_at DESC LIMIT $1`
	args := []any{limit}
	if userID != "" {
		query = `SELECT ` + orderColumns + ` FROM orders WHERE user_id = $2 ORDER BY created_at DESC LIMIT $1`
		args = append(args, userID)
	}
	return pg.Query(s.pools, func(pool *pgxpool.Pool) ([]model.Order, error) {
		return pg.All[model.Order](ctx, pool, query, args...)
	})
}

func (s *Orders) Get(ctx context.Context, id string) (model.Order, error) {
	o, err := pg.Query(s.pools, func(pool *pgxpool.Pool) (model.Order, error) {
		return pg.One[model.Order](ctx, pool, `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Order{}, model.ErrNotFound
	}
	return o, err
}

func (s *Orders) Create(ctx context.Context, o model.Order, key string) (model.Order, bool, error) {
	var idem *string
	if key != "" {
		idem = &key
	}
	out, err := pg.One[model.Order](ctx, s.pools.Write, `
		INSERT INTO orders (user_id, item, amount_cents, idempotency_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING `+orderColumns, o.UserID, o.Item, o.AmountCents, idem)
	if err == nil {
		return out, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Order{}, false, err
	}
	out, err = pg.One[model.Order](ctx, s.pools.Write, `SELECT `+orderColumns+` FROM orders WHERE idempotency_key = $1`, key)
	return out, false, err
}
