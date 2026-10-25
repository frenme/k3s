package profile

import (
	"context"
	"net/mail"
	"strings"

	"github.com/egor/k3s-base/internal/users/model"
	"github.com/google/uuid"
)

type store interface {
	List(ctx context.Context, limit int) ([]model.User, error)
	Get(ctx context.Context, id string) (model.User, error)
	Create(ctx context.Context, email, name string) (model.User, error)
}

type Service struct {
	store store
}

func NewService(store store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context, limit int) ([]model.User, error) {
	return s.store.List(ctx, limit)
}

func (s *Service) Get(ctx context.Context, id string) (model.User, error) {
	if uuid.Validate(id) != nil {
		return model.User{}, model.ErrNotFound
	}
	return s.store.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, email, name string) (model.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)
	if _, err := mail.ParseAddress(email); err != nil {
		return model.User{}, model.ValidationError("invalid email")
	}
	if name == "" {
		return model.User{}, model.ValidationError("name is required")
	}
	return s.store.Create(ctx, email, name)
}
