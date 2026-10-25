package storage

import (
	"context"
	"time"

	"github.com/egor/k3s-base/internal/admin/model"
	"github.com/egor/k3s-base/internal/lib/pg"
	"gorm.io/gorm"
)

type userRow struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
}

func (userRow) TableName() string { return "users.users" }

type Users struct {
	db *pg.DB
}

func NewUsers(db *pg.DB) *Users {
	return &Users{db: db}
}

func (s *Users) List(ctx context.Context, limit int) ([]model.User, error) {
	return pg.Read(s.db, func(db *gorm.DB) ([]model.User, error) {
		var rows []userRow
		if err := db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
			return nil, err
		}
		users := make([]model.User, 0, len(rows))
		for _, r := range rows {
			users = append(users, model.User(r))
		}
		return users, nil
	})
}
