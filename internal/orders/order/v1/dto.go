package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/orders/model"
)

type Order struct {
	ID          string    `json:"id" format:"uuid"`
	UserID      string    `json:"user_id" format:"uuid"`
	Item        string    `json:"item"`
	AmountCents int64     `json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

type ListOrdersRequest struct {
	pagination.Params
}

type ListOrdersResponse struct {
	Body pagination.Page[Order]
}

type GetOrderRequest struct {
	ID string `path:"id"`
}

type OrderResponse struct {
	Body Order
}

type CreateOrderRequest struct {
	IdempotencyKey string `header:"Idempotency-Key" maxLength:"200"`
	Body           struct {
		Item        string `json:"item" pattern:"\\S" maxLength:"200"`
		AmountCents int64  `json:"amount_cents" minimum:"0"`
	}
}

func toOrder(o model.Order) Order {
	return Order{ID: o.ID, UserID: o.UserID, Item: o.Item, AmountCents: o.AmountCents, CreatedAt: o.CreatedAt}
}
