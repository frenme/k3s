package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/egor/k3s-base/internal/platform/config"
	"github.com/egor/k3s-base/internal/platform/httpx"
	"github.com/egor/k3s-base/internal/platform/logx"
	"github.com/egor/k3s-base/internal/platform/pg"
	"github.com/egor/k3s-base/internal/users"
)

func main() {
	log := logx.New("users", config.String("LOG_LEVEL", "info"))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	schema := config.String("DB_SCHEMA", "users")
	pool, err := pg.Connect(ctx, pg.Config{
		URL:      config.String("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/app?sslmode=disable"),
		Schema:   schema,
		MaxConns: int32(config.Int("DB_MAX_CONNS", 10)),
	})
	if err != nil {
		log.Error("database connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pg.Migrate(ctx, pool, schema, users.Migrations, users.MigrationsDir); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}

	health := httpx.NewHealth()
	health.Add("postgres", pool.Ping)

	mux := http.NewServeMux()
	health.Mount(mux)
	users.NewHandler(users.NewPgStore(pool), log).Mount(mux)

	handler := httpx.Chain(mux, httpx.RequestID(), httpx.Recover(log), httpx.Observe(log))
	cfg := httpx.DefaultServerConfig(config.String("HTTP_ADDR", ":8080"))
	cfg.DrainDelay = config.Duration("DRAIN_DELAY", cfg.DrainDelay)
	cfg.GraceTimeout = config.Duration("GRACE_TIMEOUT", cfg.GraceTimeout)

	if err := httpx.Run(ctx, cfg, handler, health, log); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}
