//go:build integration

package storage

import (
	"testing"
	"testing/fstest"

	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
)

var otherSchemas = fstest.MapFS{"m/001_other_schemas.sql": {Data: []byte(`
CREATE SCHEMA users;
CREATE TABLE users.users (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text NOT NULL, name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE SCHEMA orders;
CREATE TABLE orders.orders (id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL, item text NOT NULL, amount_cents bigint NOT NULL, idempotency_key text, created_at timestamptz NOT NULL DEFAULT now());
INSERT INTO users.users (email, name) VALUES ('ann@example.com', 'Ann');
INSERT INTO orders.orders (user_id, item, amount_cents) SELECT id, 'book', 1500 FROM users.users;
`)}}

func TestReadsOtherSchemas(t *testing.T) {
	db := pgtest.DB(t, "admin", otherSchemas, "m")
	ctx := t.Context()

	users, err := NewUsers(db).List(ctx, 10)
	if err != nil || len(users) != 1 || users[0].Email != "ann@example.com" || users[0].ID == "" {
		t.Fatalf("users %+v err %v", users, err)
	}
	orders, err := NewOrders(db).List(ctx, 10)
	if err != nil || len(orders) != 1 || orders[0].UserID != users[0].ID || orders[0].AmountCents != 1500 {
		t.Fatalf("orders %+v err %v", orders, err)
	}
}
