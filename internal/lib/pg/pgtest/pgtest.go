//go:build integration

package pgtest

import (
	"context"
	"io/fs"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func URL(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("app"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(time.Minute)),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return url
}

func Pools(t *testing.T, schema string, fsys fs.FS, dir string) *pg.Pools {
	t.Helper()
	ctx := t.Context()
	url := URL(t)
	write, err := pg.Connect(ctx, pg.Config{URL: url, Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(write.Close)
	if err := pg.Migrate(ctx, write, schema, fsys, dir); err != nil {
		t.Fatal(err)
	}
	return &pg.Pools{Write: write}
}
