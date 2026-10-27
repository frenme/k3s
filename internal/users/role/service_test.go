package role

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/egor/k3s-base/internal/users/model"
	"github.com/google/uuid"
)

type fakeStore struct {
	users    map[string][]model.Role
	granted  int
	revoked  int
	failWith error
}

func (f *fakeStore) Assigned(_ context.Context, id string) ([]model.Role, error) {
	roles, ok := f.users[id]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	return roles, f.failWith
}

func (f *fakeStore) Grant(_ context.Context, id string, r model.Role) error {
	if _, ok := f.users[id]; !ok {
		return model.ErrUserNotFound
	}
	f.granted++
	if !slices.Contains(f.users[id], r) {
		f.users[id] = append(f.users[id], r)
	}
	return nil
}

func (f *fakeStore) Revoke(_ context.Context, id string, r model.Role) error {
	if _, ok := f.users[id]; !ok {
		return model.ErrUserNotFound
	}
	f.revoked++
	f.users[id] = slices.DeleteFunc(f.users[id], func(x model.Role) bool { return x == r })
	return nil
}

func newService() (*Service, string) {
	id := uuid.NewString()
	return NewService(&fakeStore{users: map[string][]model.Role{id: nil}}), id
}

func TestEveryUserHasTheUserRole(t *testing.T) {
	svc, id := newService()
	roles, err := svc.Roles(t.Context(), id)
	if err != nil || !slices.Equal(roles, []model.Role{model.RoleUser}) {
		t.Fatalf("roles %v err %v", roles, err)
	}
}

func TestGrantAndRevokeAdmin(t *testing.T) {
	svc, id := newService()
	roles, err := svc.Grant(t.Context(), id, "admin")
	if err != nil || !slices.Equal(roles, []model.Role{model.RoleAdmin, model.RoleUser}) {
		t.Fatalf("grant: %v %v", roles, err)
	}
	roles, err = svc.Grant(t.Context(), id, "admin")
	if err != nil || len(roles) != 2 {
		t.Fatalf("grant twice: %v %v", roles, err)
	}
	roles, err = svc.Revoke(t.Context(), id, "admin")
	if err != nil || !slices.Equal(roles, []model.Role{model.RoleUser}) {
		t.Fatalf("revoke: %v %v", roles, err)
	}
}

func TestUserRoleCannotBeRevoked(t *testing.T) {
	svc, id := newService()
	roles, err := svc.Revoke(t.Context(), id, "user")
	if err != nil || !slices.Contains(roles, model.RoleUser) {
		t.Fatalf("%v %v", roles, err)
	}
}

func TestErrors(t *testing.T) {
	svc, id := newService()
	if _, err := svc.Grant(t.Context(), id, "root"); !errors.Is(err, model.ErrInvalidRole) {
		t.Fatalf("invalid role: %v", err)
	}
	if _, err := svc.Roles(t.Context(), "not-a-uuid"); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("bad id: %v", err)
	}
	if _, err := svc.Roles(t.Context(), uuid.NewString()); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	if _, err := svc.Grant(t.Context(), uuid.NewString(), "admin"); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("grant to unknown user: %v", err)
	}
}
