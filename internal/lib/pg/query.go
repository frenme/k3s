package pg

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func Read[T any](db *DB, fn func(*gorm.DB) (T, error)) (T, error) {
	v, err := fn(db.Read)
	if err == nil || db.Read == db.Write {
		return v, err
	}
	return fn(db.Write)
}
