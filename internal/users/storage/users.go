package storage

import (
	"context"
	"errors"
	"time"

	"github.com/egor/k3s-base/internal/lib/cache"
	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/users/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	userColumns  = `id::text AS id, email, name, created_at`
	userCacheTTL = time.Minute
)

type Users struct {
	pools *pg.Pools
	cache *cache.Cache
}

func NewUsers(pools *pg.Pools, c *cache.Cache) *Users {
	return &Users{pools: pools, cache: c}
}

func (s *Users) List(ctx context.Context, limit int) ([]model.User, error) {
	return pg.Query(s.pools, func(pool *pgxpool.Pool) ([]model.User, error) {
		return pg.All[model.User](ctx, pool, `SELECT `+userColumns+` FROM users ORDER BY created_at DESC LIMIT $1`, limit)
	})
}

func (s *Users) Get(ctx context.Context, id string) (model.User, error) {
	return cache.Fetch(ctx, s.cache, id, userCacheTTL, func() (model.User, error) {
		u, err := pg.Query(s.pools, func(pool *pgxpool.Pool) (model.User, error) {
			return pg.One[model.User](ctx, pool, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, model.ErrNotFound
		}
		return u, err
	})
}

func (s *Users) Create(ctx context.Context, email, name string) (model.User, error) {
	u, err := pg.One[model.User](ctx, s.pools.Write, `INSERT INTO users (email, name) VALUES ($1, $2) RETURNING `+userColumns, email, name)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return model.User{}, model.ErrDuplicate
	}
	return u, err
}
