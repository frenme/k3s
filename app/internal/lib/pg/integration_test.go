//go:build integration

package pg_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
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

func TestReadFallsBackToPrimary(t *testing.T) {
	db := pgtest.DB(t, "demo", migrations, "m")
	ctx := t.Context()

	broken, err := pgxpool.New(context.Background(), "postgres://app:app@127.0.0.1:1/app?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer broken.Close()
	replica, err := pg.NewDB(&pg.Pools{Write: broken})
	if err != nil {
		t.Fatal(err)
	}
	db.Read = replica.Write

	items, err := pg.Read(ctx, db, func(g *gorm.DB) ([]item, error) {
		var out []item
		return out, g.WithContext(ctx).Table("items").Find(&out).Error
	})
	if err != nil || len(items) != 1 || items[0].Name != "one" {
		t.Fatalf("items %v err %v", items, err)
	}

	var missing item
	if err := db.Write.WithContext(ctx).Table("items").First(&missing, "id = ?", 0).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing err %v", err)
	}
}

func TestNoTransactionMigrationAndRoleSearchPath(t *testing.T) {
	ctx := t.Context()
	pool, err := pg.Connect(ctx, pg.Config{URL: pgtest.URL(t), Schema: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	files := fstest.MapFS{
		"m/001_items.sql":               {Data: []byte("CREATE TABLE items (id int PRIMARY KEY, name text NOT NULL);")},
		"m/002_items_name_idx.notx.sql": {Data: []byte("CREATE INDEX CONCURRENTLY IF NOT EXISTS items_name_idx ON items (name);")},
	}
	if err := pg.Migrate(ctx, pool, "demo", files, "m"); err != nil {
		t.Fatal(err)
	}
	if err := pg.Migrate(ctx, pool, "demo", files, "m"); err != nil {
		t.Fatal(err)
	}
	var valid bool
	if err := pool.QueryRow(ctx, "SELECT indisvalid FROM pg_index WHERE indexrelid = 'demo.items_name_idx'::regclass").Scan(&valid); err != nil || !valid {
		t.Fatalf("index valid %v err %v", valid, err)
	}

	plain, err := pgxpool.New(ctx, pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	defer plain.Close()
	var searchPath string
	if err := plain.QueryRow(ctx, "SHOW search_path").Scan(&searchPath); err != nil || searchPath != "demo" {
		t.Fatalf("search_path %q err %v", searchPath, err)
	}
}

func countItems(t *testing.T, db *pg.DB) int {
	t.Helper()
	items, err := pg.Read(t.Context(), db, func(g *gorm.DB) ([]item, error) {
		var out []item
		return out, g.WithContext(t.Context()).Table("items").Find(&out).Error
	})
	if err != nil {
		t.Fatal(err)
	}
	return len(items)
}

func insertItem(db *pg.DB, ctx context.Context, id int) error {
	return db.Conn(ctx).Exec("INSERT INTO items (id, name) VALUES (?, 'x')", id).Error
}

func TestDoCommitsAndRollsBack(t *testing.T) {
	db := pgtest.DB(t, "demo", migrations, "m")
	ctx := t.Context()
	base := countItems(t, db)

	if err := db.Do(ctx, func(ctx context.Context) error {
		if err := insertItem(db, ctx, 10); err != nil {
			return err
		}
		return insertItem(db, ctx, 11)
	}); err != nil {
		t.Fatal(err)
	}
	if got := countItems(t, db); got != base+2 {
		t.Fatalf("commit: %d items, want %d", got, base+2)
	}

	boom := errors.New("second step failed")
	err := db.Do(ctx, func(ctx context.Context) error {
		if err := insertItem(db, ctx, 20); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || countItems(t, db) != base+2 {
		t.Fatalf("rollback on error: err %v items %d", err, countItems(t, db))
	}

	func() {
		defer func() { _ = recover() }()
		_ = db.Do(ctx, func(ctx context.Context) error {
			if err := insertItem(db, ctx, 30); err != nil {
				return err
			}
			panic("kaboom")
		})
	}()
	if got := countItems(t, db); got != base+2 {
		t.Fatalf("rollback on panic: %d items", got)
	}
}

func TestDoNestedJoinsOuterTransaction(t *testing.T) {
	db := pgtest.DB(t, "demo", migrations, "m")
	ctx := t.Context()
	base := countItems(t, db)
	boom := errors.New("outer failed")
	err := db.Do(ctx, func(ctx context.Context) error {
		if err := db.Do(ctx, func(ctx context.Context) error { return insertItem(db, ctx, 40) }); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) || countItems(t, db) != base {
		t.Fatalf("nested Do committed on its own: err %v items %d", err, countItems(t, db))
	}
}

func TestReadInsideDoSeesUncommittedWrites(t *testing.T) {
	db := pgtest.DB(t, "demo", migrations, "m")
	ctx := t.Context()
	broken, err := pgxpool.New(context.Background(), "postgres://app:app@127.0.0.1:1/app?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	defer broken.Close()
	replica, err := pg.NewDB(&pg.Pools{Write: broken})
	if err != nil {
		t.Fatal(err)
	}
	db.Read = replica.Write

	err = db.Do(ctx, func(ctx context.Context) error {
		if err := insertItem(db, ctx, 50); err != nil {
			return err
		}
		items, err := pg.Read(ctx, db, func(g *gorm.DB) ([]item, error) {
			var out []item
			return out, g.Table("items").Where("id = 50").Find(&out).Error
		})
		if err != nil || len(items) != 1 {
			t.Fatalf("read inside Do: %v err %v", items, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
