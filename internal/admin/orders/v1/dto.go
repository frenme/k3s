package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/admin/storage"
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
	Limit  int    `query:"limit" default:"50" minimum:"1" maximum:"500"`
	Cursor string `query:"cursor" maxLength:"200"`
}

type ListOrdersResponse struct {
	Body struct {
		Items      []Order `json:"items"`
		NextCursor string  `json:"next_cursor,omitempty"`
	}
}
