//go:build integration

package pg_test

import (
	"context"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

var migrations = fstest.MapFS{
	"m/001_items.sql": {Data: []byte("CREATE TABLE items (id int PRIMARY KEY, name text NOT NULL);")},
	"m/002_seed.sql":  {Data: []byte("INSERT INTO items VALUES (1, 'one');")},
}

type item struct {
	ID   int
	Name string
}

func TestMigrateIsIdempotentAndConcurrent(t *testing.T) {
	ctx := t.Context()
	pool, err := pg.Connect(ctx, pg.Config{URL: pgtest.URL(t), Schema: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- pg.Migrate(ctx, pool, "demo", migrations, "m")
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := pg.Migrate(ctx, pool, "demo", migrations, "m"); err != nil {
		t.Fatal(err)
	}

	var applied, rows int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM demo.schema_migrations").Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM demo.items").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if applied != 2 || rows != 1 {
		t.Fatalf("applied %d rows %d", applied, rows)
	}
}

func TestAllOneAndReplicaFallback(t *testing.T) {
	pools := pgtest.Pools(t, "demo", migrations, "m")
	ctx := t.Context()

	broken, err := pgxpool.New(context.Background(), "postgres://app:app@127.0.0.1:1/app?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer broken.Close()
	pools.Read = broken

	items, err := pg.Query(pools, func(p *pgxpool.Pool) ([]item, error) {
		return pg.All[item](ctx, p, "SELECT id, name FROM items")
	})
	if err != nil || len(items) != 1 || items[0].Name != "one" {
		t.Fatalf("items %v err %v", items, err)
	}

	empty, err := pg.All[item](ctx, pools.Write, "SELECT id, name FROM items WHERE id = 0")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty %v err %v", empty, err)
	}

	if _, err := pg.One[item](ctx, pools.Write, "SELECT id, name FROM items WHERE id = 0"); err == nil {
		t.Fatal("expected no rows error")
	}
}
