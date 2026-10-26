package storage

import (
	"context"
	"errors"
	"time"

	"github.com/egor/k3s-base/internal/lib/idempotency"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/orders/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type orderRow struct {
	ID             string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID         string `gorm:"type:uuid"`
	Item           string
	AmountCents    int64
	IdempotencyKey *string
	RequestHash    *string
	CreatedAt      time.Time
}

func (orderRow) TableName() string { return "orders" }

func (r orderRow) model() model.Order {
	return model.Order{ID: r.ID, UserID: r.UserID, Item: r.Item, AmountCents: r.AmountCents, CreatedAt: r.CreatedAt}
}

var onIdempotencyConflict = clause.OnConflict{
	Columns:     []clause.Column{{Name: "idempotency_key"}},
	TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "idempotency_key IS NOT NULL"}}},
	DoNothing:   true,
}

type Orders struct {
	db *pg.DB
}

func NewOrders(db *pg.DB) *Orders {
	return &Orders{db: db}
}

func (s *Orders) List(ctx context.Context, userID string, limit int, after *pagination.Cursor) ([]model.Order, error) {
	return pg.Read(ctx, s.db, func(db *gorm.DB) ([]model.Order, error) {
		q := db.WithContext(ctx)
		if userID != "" {
			q = q.Where("user_id = ?", userID)
		}
		var rows []orderRow
		if err := pg.Keyset(q, after, limit).Find(&rows).Error; err != nil {
			return nil, err
		}
		orders := make([]model.Order, 0, len(rows))
		for _, r := range rows {
			orders = append(orders, r.model())
		}
		return orders, nil
	})
}

func (s *Orders) Get(ctx context.Context, id string) (model.Order, error) {
	o, err := pg.Read(ctx, s.db, func(db *gorm.DB) (model.Order, error) {
		var row orderRow
		err := db.WithContext(ctx).First(&row, "id = ?", id).Error
		return row.model(), err
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Order{}, model.ErrNotFound
	}
	return o, err
}

func (s *Orders) Create(ctx context.Context, o model.Order, idem idempotency.Request) (model.Order, bool, error) {
	row := orderRow{UserID: o.UserID, Item: o.Item, AmountCents: o.AmountCents}
	if !idem.Empty() {
		row.IdempotencyKey = &idem.Key
		row.RequestHash = &idem.Fingerprint
	}
	res := s.db.Conn(ctx).Clauses(onIdempotencyConflict).Create(&row)
	if res.Error != nil {
		return model.Order{}, false, res.Error
	}
	if res.RowsAffected == 1 {
		return row.model(), true, nil
	}
	var existing orderRow
	if err := s.db.Conn(ctx).First(&existing, "idempotency_key = ?", idem.Key).Error; err != nil {
		return model.Order{}, false, err
	}
	stored := ""
	if existing.RequestHash != nil {
		stored = *existing.RequestHash
	}
	if err := idem.Verify(stored); err != nil {
		return model.Order{}, false, err
	}
	return existing.model(), false, nil
}
