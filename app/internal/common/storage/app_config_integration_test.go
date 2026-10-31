//go:build integration

package storage

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/egor/k3s-base/internal/common/model"
	"github.com/egor/k3s-base/internal/lib/cache"
	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
)

func TestAppConfig(t *testing.T) {
	db := pgtest.DB(t, "common", Migrations, MigrationsDir)
	ctx := t.Context()
	if err := db.Write.Exec(`INSERT INTO app_config (key, value) VALUES ('internal_limit', '{"max": 5}')`).Error; err != nil {
		t.Fatal(err)
	}
	store := NewAppConfig(db, nil, 0)

	public, err := store.ListPublic(ctx)
	if err != nil || len(public) != 1 || public[0].Key != "registration_enabled" || string(public[0].Value) != "true" {
		t.Fatalf("public %+v err %v", public, err)
	}
	hidden, err := store.Get(ctx, "internal_limit")
	if err != nil || hidden.Public || string(hidden.Value) != `{"max": 5}` {
		t.Fatalf("hidden %+v err %v", hidden, err)
	}
	if _, err := store.Get(ctx, "nope"); !errors.Is(err, model.ErrAppConfigNotFound) {
		t.Fatalf("missing err %v", err)
	}
}

func TestAppConfigIsCached(t *testing.T) {
	db := pgtest.DB(t, "common", Migrations, MigrationsDir)
	ctx := t.Context()
	mr := miniredis.RunT(t)
	client := cache.NewClient(mr.Addr(), "")
	t.Cleanup(func() { _ = client.Close() })
	store := NewAppConfig(db, cache.New(client, "common"), time.Minute)

	before, err := store.Get(ctx, "registration_enabled")
	if err != nil || string(before.Value) != "true" {
		t.Fatalf("get %+v err %v", before, err)
	}
	if err := db.Write.Exec(`UPDATE app_config SET value = 'false' WHERE key = 'registration_enabled'`).Error; err != nil {
		t.Fatal(err)
	}
	cached, err := store.Get(ctx, "registration_enabled")
	if err != nil || string(cached.Value) != "true" {
		t.Fatalf("expected cached value, got %+v err %v", cached, err)
	}
	mr.FastForward(2 * time.Minute)
	fresh, err := store.Get(ctx, "registration_enabled")
	if err != nil || string(fresh.Value) != "false" {
		t.Fatalf("expected refreshed value, got %+v err %v", fresh, err)
	}
}
