package storage

import (
	"context"
	"time"

	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/lib/pg"
	"gorm.io/gorm"
)

type Order struct {
	ID          string
	UserID      string
	Item        string
	AmountCents int64
	CreatedAt   time.Time
}

func (Order) TableName() string { return "orders.admin_orders" }

type Orders struct {
	db *pg.DB
}

func NewOrders(db *pg.DB) *Orders {
	return &Orders{db: db}
}

func (s *Orders) List(ctx context.Context, limit int, after *pagination.Cursor) ([]Order, error) {
	return pg.Read(ctx, s.db, func(db *gorm.DB) ([]Order, error) {
		var orders []Order
		err := pg.Keyset(db.WithContext(ctx), after, limit).Find(&orders).Error
		return orders, err
	})
}
