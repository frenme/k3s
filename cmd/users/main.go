package main

import (
	"context"
	"log/slog"
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

	if err := run(ctx, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	pools, err := pg.Open(ctx, "users", users.Migrations, users.MigrationsDir)
	if err != nil {
		return err
	}
	defer pools.Close()

	health := httpx.NewHealth()
	health.Add("postgres", pools.Write.Ping)

	mux := http.NewServeMux()
	health.Mount(mux)
	users.NewHandler(users.NewPgStore(pools), log).Mount(mux)

	return httpx.Serve(ctx, log, health, mux)
}
