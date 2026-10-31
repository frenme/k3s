package storage

import (
	"context"
	"time"

	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/lib/pg"
	"gorm.io/gorm"
)

type User struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
}

func (User) TableName() string { return "users.admin_users" }

type Users struct {
	db *pg.DB
}

func NewUsers(db *pg.DB) *Users {
	return &Users{db: db}
}

func (s *Users) List(ctx context.Context, limit int, after *pagination.Cursor) ([]User, error) {
	return pg.Read(ctx, s.db, func(db *gorm.DB) ([]User, error) {
		var users []User
		err := pg.Keyset(db.WithContext(ctx), after, limit).Find(&users).Error
		return users, err
	})
}
