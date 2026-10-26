package profile

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/pagination"

	"github.com/egor/k3s-base/internal/users/model"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var registered = promauto.NewCounter(prometheus.CounterOpts{
	Name: "users_registered_total",
	Help: "Users created.",
})

type store interface {
	List(ctx context.Context, limit int, after *pagination.Cursor) ([]model.User, error)
	Get(ctx context.Context, id string) (model.User, error)
	Create(ctx context.Context, email, name string) (model.User, error)
}

type appConfigSource interface {
	Bool(ctx context.Context, key string) (bool, error)
}

var (
	ErrRegistrationClosed   = httpx.Error{Status: http.StatusForbidden, Code: "registration_closed", Message: "Registration is closed"}
	ErrAppConfigUnavailable = httpx.Error{Status: http.StatusBadGateway, Code: "app_config_unavailable", Message: "AppConfig is temporarily unavailable"}
)

type Service struct {
	store store
	cfg   appConfigSource
}

func NewService(store store, cfg appConfigSource) *Service {
	return &Service{store: store, cfg: cfg}
}

func (s *Service) List(ctx context.Context, limit int, cursor string) (pagination.Page[model.User], error) {
	after, err := pagination.Decode(cursor)
	if err != nil {
		return pagination.Page[model.User]{}, err
	}
	users, err := s.store.List(ctx, limit, after)
	if err != nil {
		return pagination.Page[model.User]{}, err
	}
	return pagination.NewPage(users, limit, func(u model.User) pagination.Cursor {
		return pagination.Cursor{CreatedAt: u.CreatedAt, ID: u.ID}
	}), nil
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
		return model.User{}, fmt.Errorf("%w: %w", ErrAppConfigUnavailable, err)
	}
	if !open {
		return model.User{}, ErrRegistrationClosed
	}
	u, err := s.store.Create(ctx, strings.TrimSpace(strings.ToLower(email)), strings.TrimSpace(name))
	if err == nil {
		registered.Inc()
	}
	return u, err
}
