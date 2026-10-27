//go:build integration

package storage

import (
	"errors"
	"testing"

	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
	"github.com/egor/k3s-base/internal/users/model"
	"github.com/google/uuid"
)

func TestUsers(t *testing.T) {
	store := NewUsers(pgtest.DB(t, "users", Migrations, MigrationsDir), nil)
	ctx := t.Context()

	empty, err := store.List(ctx, 10, nil)
	if err != nil || len(empty) != 1 || empty[0].Email != "admin@example.com" {
		t.Fatalf("empty list %v err %v", empty, err)
	}

	ann, err := store.Create(ctx, "ann@example.com", "Ann")
	if err != nil || uuid.Validate(ann.ID) != nil || ann.CreatedAt.IsZero() {
		t.Fatalf("create %+v err %v", ann, err)
	}
	if _, err := store.Create(ctx, "ann@example.com", "Other"); !errors.Is(err, model.ErrEmailTaken) {
		t.Fatalf("duplicate err %v", err)
	}
	if _, err := store.Create(ctx, "bob@example.com", "Bob"); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, ann.ID)
	if err != nil || got.Email != "ann@example.com" {
		t.Fatalf("get %+v err %v", got, err)
	}
	if _, err := store.Get(ctx, uuid.NewString()); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("missing err %v", err)
	}

	list, err := store.List(ctx, 1, nil)
	if err != nil || len(list) != 2 || list[0].Email != "bob@example.com" {
		t.Fatalf("list %+v err %v", list, err)
	}
	next, err := store.List(ctx, 1, &pagination.Cursor{CreatedAt: list[0].CreatedAt, ID: list[0].ID})
	if err != nil || len(next) != 2 || next[0].Email != "ann@example.com" || next[1].Email != "admin@example.com" {
		t.Fatalf("next page %+v err %v", next, err)
	}
}

func TestRoles(t *testing.T) {
	db := pgtest.DB(t, "users", Migrations, MigrationsDir)
	users, roles := NewUsers(db, nil), NewRoles(db)
	ctx := t.Context()

	ann, err := users.Create(ctx, "roles-ann@example.com", "Ann")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := roles.Assigned(ctx, ann.ID); err != nil || len(got) != 0 {
		t.Fatalf("fresh user: %v %v", got, err)
	}
	if err := roles.Grant(ctx, ann.ID, model.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := roles.Grant(ctx, ann.ID, model.RoleAdmin); err != nil {
		t.Fatalf("granting twice must be harmless: %v", err)
	}
	if got, err := roles.Assigned(ctx, ann.ID); err != nil || len(got) != 1 || got[0] != model.RoleAdmin {
		t.Fatalf("after grant: %v %v", got, err)
	}
	if err := roles.Revoke(ctx, ann.ID, model.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if got, _ := roles.Assigned(ctx, ann.ID); len(got) != 0 {
		t.Fatalf("after revoke: %v", got)
	}

	missing := uuid.NewString()
	if _, err := roles.Assigned(ctx, missing); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("assigned for a missing user: %v", err)
	}
	if err := roles.Grant(ctx, missing, model.RoleAdmin); !errors.Is(err, model.ErrUserNotFound) {
		t.Fatalf("grant for a missing user: %v", err)
	}
}

func TestSeededAdmin(t *testing.T) {
	db := pgtest.DB(t, "users", Migrations, MigrationsDir)
	got, err := NewRoles(db).Assigned(t.Context(), "00000000-0000-4000-8000-000000000001")
	if err != nil || len(got) != 1 || got[0] != model.RoleAdmin {
		t.Fatalf("seeded admin: %v %v", got, err)
	}
}
