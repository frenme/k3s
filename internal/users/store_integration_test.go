//go:build integration

package users

import (
	"errors"
	"testing"

	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
	"github.com/google/uuid"
)

func TestPgStore(t *testing.T) {
	store := NewPgStore(pgtest.Pools(t, "users", Migrations, MigrationsDir), nil)
	ctx := t.Context()

	empty, err := store.List(ctx, 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list %v err %v", empty, err)
	}

	ann, err := store.Create(ctx, "ann@example.com", "Ann")
	if err != nil || uuid.Validate(ann.ID) != nil || ann.CreatedAt.IsZero() {
		t.Fatalf("create %+v err %v", ann, err)
	}
	if _, err := store.Create(ctx, "ann@example.com", "Other"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate err %v", err)
	}
	if _, err := store.Create(ctx, "bob@example.com", "Bob"); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(ctx, ann.ID)
	if err != nil || got.Email != "ann@example.com" {
		t.Fatalf("get %+v err %v", got, err)
	}
	if _, err := store.Get(ctx, uuid.NewString()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err %v", err)
	}

	list, err := store.List(ctx, 1)
	if err != nil || len(list) != 1 || list[0].Email != "bob@example.com" {
		t.Fatalf("list %+v err %v", list, err)
	}
}
