package orders

import (
	"context"
	"embed"
	"errors"
	"time"

	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var Migrations embed.FS

const MigrationsDir = "migrations"

var ErrNotFound = errors.New("order not found")

type Order struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Item        string    `json:"item"`
	AmountCents int64     `json:"amount_cents" db:"amount_cents"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type Store interface {
	List(ctx context.Context, userID string, limit int) ([]Order, error)
	Get(ctx context.Context, id string) (Order, error)
	Create(ctx context.Context, o Order, idempotencyKey string) (Order, bool, error)
}

type PgStore struct {
	pools *pg.Pools
}

func NewPgStore(pools *pg.Pools) *PgStore {
	return &PgStore{pools: pools}
}

const orderColumns = `id::text AS id, user_id::text AS user_id, item, amount_cents, created_at`

func (s *PgStore) List(ctx context.Context, userID string, limit int) ([]Order, error) {
	query := `SELECT ` + orderColumns + ` FROM orders ORDER BY created_at DESC LIMIT $1`
	args := []any{limit}
	if userID != "" {
		query = `SELECT ` + orderColumns + ` FROM orders WHERE user_id = $2 ORDER BY created_at DESC LIMIT $1`
		args = append(args, userID)
	}
	return pg.Query(s.pools, func(pool *pgxpool.Pool) ([]Order, error) {
		return pg.All[Order](ctx, pool, query, args...)
	})
}

func (s *PgStore) Create(ctx context.Context, o Order, key string) (Order, bool, error) {
	var idem *string
	if key != "" {
		idem = &key
	}
	out, err := pg.One[Order](ctx, s.pools.Write, `
		INSERT INTO orders (user_id, item, amount_cents, idempotency_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING `+orderColumns, o.UserID, o.Item, o.AmountCents, idem)
	if err == nil {
		return out, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Order{}, false, err
	}
	out, err = pg.One[Order](ctx, s.pools.Write, `SELECT `+orderColumns+` FROM orders WHERE idempotency_key = $1`, key)
	return out, false, err
}

func (s *PgStore) Get(ctx context.Context, id string) (Order, error) {
	o, err := pg.Query(s.pools, func(pool *pgxpool.Pool) (Order, error) {
		return pg.One[Order](ctx, pool, `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	return o, err
}
