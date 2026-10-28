package order

import (
	"context"
	"errors"
	"strings"

	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/lib/idempotency"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var ordersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "orders_created_total",
	Help: "Order creation requests by result.",
}, []string{"result"})

type store interface {
	List(ctx context.Context, userID string, limit int, after *pagination.Cursor) ([]model.Order, error)
	Get(ctx context.Context, id string) (model.Order, error)
	Create(ctx context.Context, o model.Order, idem idempotency.Request) (model.Order, bool, error)
}

type userDirectory interface {
	Get(ctx context.Context, id string) (users.User, error)
}

type Service struct {
	store store
	users userDirectory
}

func NewService(store store, users userDirectory) *Service {
	return &Service{store: store, users: users}
}

func (s *Service) List(ctx context.Context, userID string, p pagination.Params) (pagination.Page[model.Order], error) {
	find := func(ctx context.Context, limit int, after *pagination.Cursor) ([]model.Order, error) {
		return s.store.List(ctx, userID, limit, after)
	}
	return pagination.Fetch(ctx, p, find, func(o model.Order) pagination.Cursor {
		return pagination.Cursor{CreatedAt: o.CreatedAt, ID: o.ID}
	})
}

func (s *Service) Get(ctx context.Context, userID, id string) (model.Order, error) {
	o, err := s.store.Get(ctx, id)
	if err != nil {
		return model.Order{}, err
	}
	if !strings.EqualFold(o.UserID, userID) {
		return model.Order{}, model.ErrOrderNotFound
	}
	return o, nil
}

func (s *Service) Create(ctx context.Context, o model.Order, idem idempotency.Request) (model.Order, bool, error) {
	o.Item = strings.TrimSpace(o.Item)
	if _, err := s.users.Get(ctx, o.UserID); errors.Is(err, users.ErrNotFound) {
		return model.Order{}, false, model.ErrUnknownUser
	} else if err != nil {
		return model.Order{}, false, err
	}
	created, isNew, err := s.store.Create(ctx, o, idem)
	if err != nil {
		return model.Order{}, false, err
	}
	if isNew {
		ordersTotal.WithLabelValues("created").Inc()
	} else {
		ordersTotal.WithLabelValues("replayed").Inc()
	}
	return created, isNew, nil
}
