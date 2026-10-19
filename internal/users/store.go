package users

import (
	"context"
	"embed"
	"errors"
	"time"

	"github.com/egor/k3s-base/internal/lib/cache"
	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var Migrations embed.FS

const MigrationsDir = "migrations"

const userCacheTTL = time.Minute

var (
	ErrNotFound  = errors.New("user not found")
	ErrDuplicate = errors.New("email already exists")
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Store interface {
	List(ctx context.Context, limit int) ([]User, error)
	Get(ctx context.Context, id string) (User, error)
	Create(ctx context.Context, email, name string) (User, error)
}

type PgStore struct {
	pools *pg.Pools
	cache *cache.Cache
}

func NewPgStore(pools *pg.Pools, c *cache.Cache) *PgStore {
	return &PgStore{pools: pools, cache: c}
}

const userColumns = `id::text AS id, email, name, created_at`

func (s *PgStore) List(ctx context.Context, limit int) ([]User, error) {
	return pg.Query(s.pools, func(pool *pgxpool.Pool) ([]User, error) {
		return pg.All[User](ctx, pool, `SELECT `+userColumns+` FROM users ORDER BY created_at DESC LIMIT $1`, limit)
	})
}

func (s *PgStore) Get(ctx context.Context, id string) (User, error) {
	return cache.Fetch(ctx, s.cache, id, userCacheTTL, func() (User, error) {
		return s.load(ctx, id)
	})
}

func (s *PgStore) load(ctx context.Context, id string) (User, error) {
	u, err := pg.Query(s.pools, func(pool *pgxpool.Pool) (User, error) {
		return pg.One[User](ctx, pool, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (s *PgStore) Create(ctx context.Context, email, name string) (User, error) {
	u, err := pg.One[User](ctx, s.pools.Write, `INSERT INTO users (email, name) VALUES ($1, $2) RETURNING `+userColumns, email, name)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return User{}, ErrDuplicate
	}
	return u, err
}
