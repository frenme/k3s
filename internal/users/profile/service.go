package profile

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/egor/k3s-base/internal/lib/httpx"

	"github.com/egor/k3s-base/internal/users/model"
	"github.com/google/uuid"
)

type store interface {
	List(ctx context.Context, limit int) ([]model.User, error)
	Get(ctx context.Context, id string) (model.User, error)
	Create(ctx context.Context, email, name string) (model.User, error)
}

type configSource interface {
	Bool(ctx context.Context, key string) (bool, error)
}

var (
	ErrRegistrationClosed = httpx.Error{Status: http.StatusForbidden, Code: "registration_closed", Message: "Registration is closed"}
	ErrConfigUnavailable  = httpx.Error{Status: http.StatusBadGateway, Code: "config_unavailable", Message: "Config is temporarily unavailable"}
)

type Service struct {
	store store
	cfg   configSource
}

func NewService(store store, cfg configSource) *Service {
	return &Service{store: store, cfg: cfg}
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
	open, err := s.cfg.Bool(ctx, "registration_enabled")
	if err != nil {
		return model.User{}, fmt.Errorf("%w: %w", ErrConfigUnavailable, err)
	}
	if !open {
		return model.User{}, ErrRegistrationClosed
	}
	return s.store.Create(ctx, strings.TrimSpace(strings.ToLower(email)), strings.TrimSpace(name))
}
