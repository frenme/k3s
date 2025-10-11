package orders

import (
	"context"
	"embed"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var Migrations embed.FS

const MigrationsDir = "migrations"

type Order struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Item        string    `json:"item"`
	AmountCents int64     `json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

type Store interface {
	List(ctx context.Context, userID string, limit int) ([]Order, error)
	Create(ctx context.Context, o Order, idempotencyKey string) (Order, bool, error)
}

type PgStore struct {
	pool *pgxpool.Pool
}

func NewPgStore(pool *pgxpool.Pool) *PgStore {
	return &PgStore{pool: pool}
}

const orderColumns = `id::text, user_id::text, item, amount_cents, created_at`

func (s *PgStore) List(ctx context.Context, userID string, limit int) ([]Order, error) {
	query := `SELECT ` + orderColumns + ` FROM orders ORDER BY created_at DESC LIMIT $1`
	args := []any{limit}
	if userID != "" {
		query = `SELECT ` + orderColumns + ` FROM orders WHERE user_id = $2 ORDER BY created_at DESC LIMIT $1`
		args = append(args, userID)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Item, &o.AmountCents, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

