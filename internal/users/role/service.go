package role

import (
	"context"
	"slices"

	"github.com/egor/k3s-base/internal/users/model"
	"github.com/google/uuid"
)

type store interface {
	Assigned(ctx context.Context, userID string) ([]model.Role, error)
	Grant(ctx context.Context, userID string, role model.Role) error
	Revoke(ctx context.Context, userID string, role model.Role) error
}

type Service struct {
	store store
}

func NewService(store store) *Service {
	return &Service{store: store}
}

func (s *Service) Roles(ctx context.Context, userID string) ([]model.Role, error) {
	if uuid.Validate(userID) != nil {
		return nil, model.ErrUserNotFound
	}
	assigned, err := s.store.Assigned(ctx, userID)
	if err != nil {
		return nil, err
	}
	roles := append([]model.Role{model.RoleUser}, assigned...)
	slices.Sort(roles)
	return slices.Compact(roles), nil
}

func (s *Service) Grant(ctx context.Context, userID, role string) ([]model.Role, error) {
	return s.change(ctx, userID, role, s.store.Grant)
}

func (s *Service) Revoke(ctx context.Context, userID, role string) ([]model.Role, error) {
	return s.change(ctx, userID, role, s.store.Revoke)
}

func (s *Service) change(ctx context.Context, userID, role string, apply func(context.Context, string, model.Role) error) ([]model.Role, error) {
	parsed, ok := model.ParseRole(role)
	if !ok {
		return nil, model.ErrInvalidRole
	}
	if uuid.Validate(userID) != nil {
		return nil, model.ErrUserNotFound
	}
	if err := apply(ctx, userID, parsed); err != nil {
		return nil, err
	}
	return s.Roles(ctx, userID)
}
