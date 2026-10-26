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
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list %v err %v", empty, err)
	}

	ann, err := store.Create(ctx, "ann@example.com", "Ann")
	if err != nil || uuid.Validate(ann.ID) != nil || ann.CreatedAt.IsZero() {
		t.Fatalf("create %+v err %v", ann, err)
	}
	if _, err := store.Create(ctx, "ann@example.com", "Other"); !errors.Is(err, model.ErrDuplicate) {
		t.Fatalf("duplicate err %v", err)
	}
	if _, err := store.Create(ctx, "bob@example.com", "Bob"); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, ann.ID)
	if err != nil || got.Email != "ann@example.com" {
		t.Fatalf("get %+v err %v", got, err)
	}
	if _, err := store.Get(ctx, uuid.NewString()); !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("missing err %v", err)
	}

	list, err := store.List(ctx, 1, nil)
	if err != nil || len(list) != 2 || list[0].Email != "bob@example.com" {
		t.Fatalf("list %+v err %v", list, err)
	}
	next, err := store.List(ctx, 1, &pagination.Cursor{CreatedAt: list[0].CreatedAt, ID: list[0].ID})
	if err != nil || len(next) != 1 || next[0].Email != "ann@example.com" {
		t.Fatalf("next page %+v err %v", next, err)
	}
}
