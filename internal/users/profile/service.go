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

type settings interface {
	Bool(ctx context.Context, key string) (bool, error)
}

var (
	ErrRegistrationClosed  = httpx.Error{Status: http.StatusForbidden, Code: "registration_closed", Message: "Registration is closed"}
	ErrSettingsUnavailable = httpx.Error{Status: http.StatusBadGateway, Code: "settings_unavailable", Message: "Settings are temporarily unavailable"}
)

type Service struct {
	store    store
	settings settings
}

func NewService(store store, settings settings) *Service {
	return &Service{store: store, settings: settings}
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
	open, err := s.settings.Bool(ctx, "registration_enabled")
	if err != nil {
		return model.User{}, fmt.Errorf("%w: %w", ErrSettingsUnavailable, err)
	}
	if !open {
		return model.User{}, ErrRegistrationClosed
	}
	return s.store.Create(ctx, strings.TrimSpace(strings.ToLower(email)), strings.TrimSpace(name))
}
