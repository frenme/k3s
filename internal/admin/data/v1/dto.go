package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/admin/model"
)

type User struct {
	ID        string    `json:"id" format:"uuid"`
	Email     string    `json:"email" format:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Order struct {
	ID          string    `json:"id" format:"uuid"`
	UserID      string    `json:"user_id" format:"uuid"`
	Item        string    `json:"item"`
	AmountCents int64     `json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

func toUser(u model.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}

func toOrder(o model.Order) Order {
	return Order{ID: o.ID, UserID: o.UserID, Item: o.Item, AmountCents: o.AmountCents, CreatedAt: o.CreatedAt}
}

type ListRequest struct {
	Limit int `query:"limit" default:"50" minimum:"1" maximum:"500"`
}

type ListUsersResponse struct {
	Body struct {
		Items []User `json:"items"`
	}
}

type ListOrdersResponse struct {
	Body struct {
		Items []Order `json:"items"`
	}
}
