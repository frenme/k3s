package checkout

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/google/uuid"
)

var ErrUsersUnavailable = errors.New("users service unavailable")

type store interface {
	List(ctx context.Context, userID string, limit int) ([]model.Order, error)
	Get(ctx context.Context, id string) (model.Order, error)
	Create(ctx context.Context, o model.Order, idempotencyKey string) (model.Order, bool, error)
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

func (s *Service) List(ctx context.Context, userID string, limit int) ([]model.Order, error) {
	if userID != "" && uuid.Validate(userID) != nil {
		return nil, model.ValidationError("invalid user_id")
	}
	return s.store.List(ctx, userID, limit)
}

func (s *Service) Get(ctx context.Context, id string) (model.Order, error) {
	if uuid.Validate(id) != nil {
		return model.Order{}, model.ErrNotFound
	}
	return s.store.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, o model.Order, idempotencyKey string) (model.Order, bool, error) {
	o.Item = strings.TrimSpace(o.Item)
	if o.UserID == "" || o.Item == "" || o.AmountCents < 0 {
		return model.Order{}, false, model.ValidationError("user_id, item and non-negative amount_cents are required")
	}
	if uuid.Validate(o.UserID) != nil {
		return model.Order{}, false, model.ValidationError("invalid user_id")
	}
	if _, err := s.users.Get(ctx, o.UserID); errors.Is(err, users.ErrNotFound) {
		return model.Order{}, false, model.ErrUnknownUser
	} else if err != nil {
		return model.Order{}, false, fmt.Errorf("%w: %w", ErrUsersUnavailable, err)
	}
	return s.store.Create(ctx, o, idempotencyKey)
}
