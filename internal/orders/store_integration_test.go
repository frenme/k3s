//go:build integration

package orders

import (
	"errors"
	"testing"

	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
	"github.com/google/uuid"
)

func TestPgStore(t *testing.T) {
	store := NewPgStore(pgtest.Pools(t, "orders", Migrations, MigrationsDir))
	ctx := t.Context()
	user, other := uuid.NewString(), uuid.NewString()

	first, created, err := store.Create(ctx, Order{UserID: user, Item: "book", AmountCents: 1500}, "key-1")
	if err != nil || !created || first.ID == "" {
		t.Fatalf("create %+v created %v err %v", first, created, err)
	}
	replay, created, err := store.Create(ctx, Order{UserID: user, Item: "other", AmountCents: 1}, "key-1")
	if err != nil || created || replay.ID != first.ID || replay.Item != "book" {
		t.Fatalf("replay %+v created %v err %v", replay, created, err)
	}
	if _, created, err := store.Create(ctx, Order{UserID: other, Item: "pen", AmountCents: 100}, ""); err != nil || !created {
		t.Fatalf("no key created %v err %v", created, err)
	}

	mine, err := store.List(ctx, user, 10)
	if err != nil || len(mine) != 1 || mine[0].ID != first.ID {
		t.Fatalf("list by user %+v err %v", mine, err)
	}
	all, err := store.List(ctx, "", 10)
	if err != nil || len(all) != 2 {
		t.Fatalf("list all %+v err %v", all, err)
	}

	got, err := store.Get(ctx, first.ID)
	if err != nil || got.AmountCents != 1500 || got.UserID != user {
		t.Fatalf("get %+v err %v", got, err)
	}
	if _, err := store.Get(ctx, uuid.NewString()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err %v", err)
	}
}
