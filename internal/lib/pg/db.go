package pg

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	Write *gorm.DB
	Read  *gorm.DB
}

func NewDB(p *Pools) (*DB, error) {
	write, err := openGorm(p.Write)
	if err != nil {
		return nil, err
	}
	db := &DB{Write: write, Read: write}
	if p.Read != nil {
		if db.Read, err = openGorm(p.Read); err != nil {
			return nil, err
		}
	}
	return db, nil
}

func openGorm(pool *pgxpool.Pool) (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{Conn: stdlib.OpenDBFromPool(pool)}), &gorm.Config{
		TranslateError:         true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Discard,
	})
}

type txKey struct{}

func (db *DB) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return db.Write.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

func (db *DB) Conn(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return db.Write.WithContext(ctx)
}

func Read[T any](ctx context.Context, db *DB, fn func(*gorm.DB) (T, error)) (T, error) {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(tx)
	}
	v, err := fn(db.Read)
	if err == nil || !db.HasReplica() || !fallbackToPrimary(err) {
		return v, err
	}
	return fn(db.Write)
}

func fallbackToPrimary(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true
	}
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		switch pgErr.Code[:2] {
		case "08", "40", "53", "57", "58", "XX":
			return true
		}
		return false
	}
	return true
}

func (db *DB) HasReplica() bool {
	return db.Read != db.Write
}

func (db *DB) PingWrite(ctx context.Context) error {
	return db.Write.WithContext(ctx).Exec("SELECT 1").Error
}

func (db *DB) PingRead(ctx context.Context) error {
	return db.Read.WithContext(ctx).Exec("SELECT 1").Error
}
