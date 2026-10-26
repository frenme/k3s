//go:build integration

package storage

import (
	"errors"
	"testing"

	"github.com/egor/k3s-base/internal/common/model"
	"github.com/egor/k3s-base/internal/lib/pg/pgtest"
)

func TestConfig(t *testing.T) {
	db := pgtest.DB(t, "common", Migrations, MigrationsDir)
	ctx := t.Context()
	if err := db.Write.Exec(`INSERT INTO config (key, value) VALUES ('internal_limit', '{"max": 5}')`).Error; err != nil {
		t.Fatal(err)
	}
	store := NewConfig(db)

	public, err := store.ListPublic(ctx)
	if err != nil || len(public) != 1 || public[0].Key != "registration_enabled" || string(public[0].Value) != "true" {
		t.Fatalf("public %+v err %v", public, err)
	}
	hidden, err := store.Get(ctx, "internal_limit")
	if err != nil || hidden.Public || string(hidden.Value) != `{"max": 5}` {
		t.Fatalf("hidden %+v err %v", hidden, err)
	}
	if _, err := store.Get(ctx, "nope"); !errors.Is(err, model.ErrConfigNotFound) {
		t.Fatalf("missing err %v", err)
	}
}
