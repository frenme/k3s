package config

import (
	"context"

	"github.com/egor/k3s-base/internal/common/model"
)

type store interface {
	ListPublic(ctx context.Context) ([]model.Config, error)
	Get(ctx context.Context, key string) (model.Config, error)
}

type Service struct {
	store store
}

func NewService(store store) *Service {
	return &Service{store: store}
}

func (s *Service) Public(ctx context.Context) ([]model.Config, error) {
	return s.store.ListPublic(ctx)
}

func (s *Service) Get(ctx context.Context, key string) (model.Config, error) {
	return s.store.Get(ctx, key)
}
