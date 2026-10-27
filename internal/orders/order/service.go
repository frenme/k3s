package order

import (
	"context"
	"errors"
	"strings"

	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/lib/idempotency"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/google/uuid"
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

func (s *Service) List(ctx context.Context, userID string, limit int, cursor string) (pagination.Page[model.Order], error) {
	after, err := pagination.Decode(cursor)
	if err != nil {
		return pagination.Page[model.Order]{}, err
	}
	orders, err := s.store.List(ctx, userID, limit, after)
	if err != nil {
		return pagination.Page[model.Order]{}, err
	}
	return pagination.NewPage(orders, limit, func(o model.Order) pagination.Cursor {
		return pagination.Cursor{CreatedAt: o.CreatedAt, ID: o.ID}
	}), nil
}

func (s *Service) Get(ctx context.Context, id string) (model.Order, error) {
	if uuid.Validate(id) != nil {
		return model.Order{}, model.ErrOrderNotFound
	}
	return s.store.Get(ctx, id)
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
