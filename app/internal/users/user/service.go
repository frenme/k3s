package user

import (
	"context"
	"strings"

	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/users/model"
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

type Service struct {
	store store
	cfg   appConfigSource
}

func NewService(store store, cfg appConfigSource) *Service {
	return &Service{store: store, cfg: cfg}
}

func (s *Service) List(ctx context.Context, p pagination.Params) (pagination.Page[model.User], error) {
	return pagination.Fetch(ctx, p, s.store.List, func(u model.User) pagination.Cursor {
		return pagination.Cursor{CreatedAt: u.CreatedAt, ID: u.ID}
	})
}

func (s *Service) Get(ctx context.Context, id string) (model.User, error) {
	return s.store.Get(ctx, id)
}

func (s *Service) GetSelf(ctx context.Context, viewerID, id string) (model.User, error) {
	if !strings.EqualFold(viewerID, id) {
		return model.User{}, model.ErrUserNotFound
	}
	return s.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, email, name string) (model.User, error) {
	open, err := s.cfg.Bool(ctx, "registration_enabled")
	if err != nil {
		return model.User{}, err
	}
	if !open {
		return model.User{}, model.ErrRegistrationClosed
	}
	u, err := s.store.Create(ctx, strings.TrimSpace(strings.ToLower(email)), strings.TrimSpace(name))
	if err == nil {
		registered.Inc()
	}
	return u, err
}
