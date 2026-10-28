package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/admin/storage"
	"github.com/egor/k3s-base/internal/lib/pagination"
)

type Order struct {
	ID          string    `json:"id" format:"uuid"`
	UserID      string    `json:"user_id" format:"uuid"`
	Item        string    `json:"item"`
	AmountCents int64     `json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

func toOrder(o storage.Order) Order {
	return Order{ID: o.ID, UserID: o.UserID, Item: o.Item, AmountCents: o.AmountCents, CreatedAt: o.CreatedAt}
}

type ListRequest struct {
	pagination.Params
}

type ListOrdersResponse struct {
	Body pagination.Page[Order]
}
