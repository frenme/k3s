package data

import (
	"context"

	"github.com/egor/k3s-base/internal/admin/model"
)

type users interface {
	List(ctx context.Context, limit int) ([]model.User, error)
}

type orders interface {
	List(ctx context.Context, limit int) ([]model.Order, error)
}

type Service struct {
	users  users
	orders orders
}

func NewService(users users, orders orders) *Service {
	return &Service{users: users, orders: orders}
}

func (s *Service) Users(ctx context.Context, limit int) ([]model.User, error) {
	return s.users.List(ctx, limit)
}

func (s *Service) Orders(ctx context.Context, limit int) ([]model.Order, error) {
	return s.orders.List(ctx, limit)
}
