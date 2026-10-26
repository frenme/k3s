package storage

import (
	"context"
	"time"

	"github.com/egor/k3s-base/internal/admin/model"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/lib/pg"
	"gorm.io/gorm"
)

type userRow struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
}

func (userRow) TableName() string { return "users.admin_users" }

type Users struct {
	db *pg.DB
}

func NewUsers(db *pg.DB) *Users {
	return &Users{db: db}
}

func (s *Users) List(ctx context.Context, limit int, after *pagination.Cursor) ([]model.User, error) {
	return pg.Read(ctx, s.db, func(db *gorm.DB) ([]model.User, error) {
		var rows []userRow
		if err := pg.Keyset(db.WithContext(ctx), after, limit).Find(&rows).Error; err != nil {
			return nil, err
		}
		users := make([]model.User, 0, len(rows))
		for _, r := range rows {
			users = append(users, model.User(r))
		}
		return users, nil
	})
}
