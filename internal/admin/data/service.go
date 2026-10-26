package data

import (
	"context"

	"github.com/egor/k3s-base/internal/admin/model"
	"github.com/egor/k3s-base/internal/lib/pagination"
)

type users interface {
	List(ctx context.Context, limit int, after *pagination.Cursor) ([]model.User, error)
}

type orders interface {
	List(ctx context.Context, limit int, after *pagination.Cursor) ([]model.Order, error)
}

type Service struct {
	users  users
	orders orders
}

func NewService(users users, orders orders) *Service {
	return &Service{users: users, orders: orders}
}

func (s *Service) Users(ctx context.Context, limit int, cursor string) (pagination.Page[model.User], error) {
	after, err := pagination.Decode(cursor)
	if err != nil {
		return pagination.Page[model.User]{}, err
	}
	users, err := s.users.List(ctx, limit, after)
	if err != nil {
		return pagination.Page[model.User]{}, err
	}
	return pagination.NewPage(users, limit, func(u model.User) pagination.Cursor {
		return pagination.Cursor{CreatedAt: u.CreatedAt, ID: u.ID}
	}), nil
}

func (s *Service) Orders(ctx context.Context, limit int, cursor string) (pagination.Page[model.Order], error) {
	after, err := pagination.Decode(cursor)
	if err != nil {
		return pagination.Page[model.Order]{}, err
	}
	orders, err := s.orders.List(ctx, limit, after)
	if err != nil {
		return pagination.Page[model.Order]{}, err
	}
	return pagination.NewPage(orders, limit, func(o model.Order) pagination.Cursor {
		return pagination.Cursor{CreatedAt: o.CreatedAt, ID: o.ID}
	}), nil
}
