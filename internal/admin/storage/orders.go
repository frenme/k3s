package storage

import (
	"context"
	"time"

	"github.com/egor/k3s-base/internal/admin/model"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/lib/pg"
	"gorm.io/gorm"
)

type orderRow struct {
	ID          string
	UserID      string
	Item        string
	AmountCents int64
	CreatedAt   time.Time
}

func (orderRow) TableName() string { return "orders.admin_orders" }

type Orders struct {
	db *pg.DB
}

func NewOrders(db *pg.DB) *Orders {
	return &Orders{db: db}
}

func (s *Orders) List(ctx context.Context, limit int, after *pagination.Cursor) ([]model.Order, error) {
	return pg.Read(ctx, s.db, func(db *gorm.DB) ([]model.Order, error) {
		var rows []orderRow
		if err := pg.Keyset(db.WithContext(ctx), after, limit).Find(&rows).Error; err != nil {
			return nil, err
		}
		orders := make([]model.Order, 0, len(rows))
		for _, r := range rows {
			orders = append(orders, model.Order(r))
		}
		return orders, nil
	})
}
