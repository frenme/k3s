//go:build integration

package contract

import (
	"testing"

	adminstorage "github.com/egor/k3s-base/internal/admin/storage"
	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
	orderstorage "github.com/egor/k3s-base/internal/orders/storage"
	userstorage "github.com/egor/k3s-base/internal/users/storage"
)

func TestAdminReadsServiceViews(t *testing.T) {
	ctx := t.Context()
	url := pgtest.URL(t)

	usersPool, err := pg.Connect(ctx, pg.Config{URL: url, Schema: "users"})
	if err != nil {
		t.Fatal(err)
	}
	defer usersPool.Close()
	ordersPool, err := pg.Connect(ctx, pg.Config{URL: url, Schema: "orders"})
	if err != nil {
		t.Fatal(err)
	}
	defer ordersPool.Close()
	adminPool, err := pg.Connect(ctx, pg.Config{URL: url, Schema: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()

	if err := pg.Migrate(ctx, usersPool, "users", userstorage.Migrations, userstorage.MigrationsDir); err != nil {
		t.Fatal(err)
	}
	if err := pg.Migrate(ctx, ordersPool, "orders", orderstorage.Migrations, orderstorage.MigrationsDir); err != nil {
		t.Fatal(err)
	}
	if _, err := usersPool.Exec(ctx, `INSERT INTO users (email, name) VALUES ('ann@example.com', 'Ann')`); err != nil {
		t.Fatal(err)
	}
	if _, err := ordersPool.Exec(ctx, `INSERT INTO orders (user_id, item, amount_cents) SELECT id, 'book', 1500 FROM users.users`); err != nil {
		t.Fatal(err)
	}

	db, err := pg.NewDB(&pg.Pools{Write: adminPool})
	if err != nil {
		t.Fatal(err)
	}
	users, err := adminstorage.NewUsers(db).List(ctx, 10, nil)
	if err != nil || len(users) != 1 || users[0].Email != "ann@example.com" || users[0].ID == "" {
		t.Fatalf("users %+v err %v", users, err)
	}
	orders, err := adminstorage.NewOrders(db).List(ctx, 10, nil)
	if err != nil || len(orders) != 1 || orders[0].UserID != users[0].ID || orders[0].AmountCents != 1500 {
		t.Fatalf("orders %+v err %v", orders, err)
	}
}
