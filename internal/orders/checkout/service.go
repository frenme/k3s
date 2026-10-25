package checkout

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/google/uuid"
)

var (
	ErrUnknownUser      = httpx.Error{Status: http.StatusUnprocessableEntity, Code: "unknown_user", Message: "User does not exist"}
	ErrUsersUnavailable = httpx.Error{Status: http.StatusBadGateway, Code: "users_unavailable", Message: "Users service is temporarily unavailable"}
)

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
	if _, err := s.users.Get(ctx, o.UserID); errors.Is(err, users.ErrNotFound) {
		return model.Order{}, false, ErrUnknownUser
	} else if err != nil {
		return model.Order{}, false, fmt.Errorf("%w: %w", ErrUsersUnavailable, err)
	}
	return s.store.Create(ctx, o, idempotencyKey)
}
