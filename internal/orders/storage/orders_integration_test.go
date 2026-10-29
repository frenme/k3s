//go:build integration

package storage

import (
	"errors"
	"testing"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/idempotency"
	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/google/uuid"
)

func TestOrders(t *testing.T) {
	store := NewOrders(pgtest.DB(t, "orders", Migrations, MigrationsDir))
	ctx := t.Context()
	user, other := uuid.NewString(), uuid.NewString()

	body := model.Order{UserID: user, Item: "book", AmountCents: 1500}
	idem, _ := idempotency.New(user, "key-1", body)
	first, created, err := store.Create(ctx, body, idem)
	if err != nil || !created || first.ID == "" || first.CreatedAt.IsZero() {
		t.Fatalf("create %+v created %v err %v", first, created, err)
	}
	replay, created, err := store.Create(ctx, body, idem)
	if err != nil || created || replay.ID != first.ID || replay.Item != "book" {
		t.Fatalf("replay %+v created %v err %v", replay, created, err)
	}
	changed := body
	changed.AmountCents = 1
	changedIdem, _ := idempotency.New(user, "key-1", changed)
	if _, _, err := store.Create(ctx, changed, changedIdem); !errors.Is(err, httpx.ErrIdempotencyKeyReused) {
		t.Fatalf("changed body err %v", err)
	}
	otherIdem, _ := idempotency.New(other, "key-1", body)
	if o, created, err := store.Create(ctx, model.Order{UserID: other, Item: "book", AmountCents: 1500}, otherIdem); err != nil || !created || o.ID == first.ID {
		t.Fatalf("same key for another user %+v created %v err %v", o, created, err)
	}
	if _, created, err := store.Create(ctx, model.Order{UserID: other, Item: "pen", AmountCents: 100}, idempotency.Request{}); err != nil || !created {
		t.Fatalf("no key created %v err %v", created, err)
	}

	mine, err := store.List(ctx, user, 10, nil)
	if err != nil || len(mine) != 1 || mine[0].ID != first.ID {
		t.Fatalf("list by user %+v err %v", mine, err)
	}
	theirs, err := store.List(ctx, other, 10, nil)
	if err != nil || len(theirs) != 2 {
		t.Fatalf("list of another user %+v err %v", theirs, err)
	}

	got, err := store.Get(ctx, first.ID)
	if err != nil || got.AmountCents != 1500 || got.UserID != user {
		t.Fatalf("get %+v err %v", got, err)
	}
	if _, err := store.Get(ctx, uuid.NewString()); !errors.Is(err, model.ErrOrderNotFound) {
		t.Fatalf("missing err %v", err)
	}
	if _, err := store.Get(ctx, "not-a-uuid"); !errors.Is(err, model.ErrOrderNotFound) {
		t.Fatalf("not a uuid err %v", err)
	}
	if none, err := store.List(ctx, "not-a-uuid", 10, nil); err != nil || len(none) != 0 {
		t.Fatalf("list for a user that is not a uuid %+v err %v", none, err)
	}
}
