package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/orders/model"
)

type Order struct {
	ID          string    `json:"id" format:"uuid"`
	UserID      string    `json:"user_id" format:"uuid"`
	Item        string    `json:"item"`
	AmountCents int64     `json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

func toOrder(o model.Order) Order {
	return Order{ID: o.ID, UserID: o.UserID, Item: o.Item, AmountCents: o.AmountCents, CreatedAt: o.CreatedAt}
}

type ListOrdersRequest struct {
	Limit  int    `query:"limit" default:"50" minimum:"1" maximum:"500"`
	UserID string `query:"user_id" format:"uuid"`
}

type ListOrdersResponse struct {
	Body struct {
		Items []Order `json:"items"`
	}
}

type GetOrderRequest struct {
	ID string `path:"id"`
}

type OrderResponse struct {
	Status int
	Body   Order
}

type CreateOrderRequest struct {
	IdempotencyKey string `header:"Idempotency-Key" maxLength:"200"`
	Body           struct {
		UserID      string `json:"user_id" format:"uuid"`
		Item        string `json:"item" pattern:"\\S" maxLength:"200"`
		AmountCents int64  `json:"amount_cents" minimum:"0"`
	}
}
