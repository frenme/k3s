package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/egor/k3s-base/internal/orders"
	"github.com/egor/k3s-base/internal/platform/config"
	"github.com/egor/k3s-base/internal/platform/httpx"
	"github.com/egor/k3s-base/internal/platform/logx"
	"github.com/egor/k3s-base/internal/platform/pg"
)

func main() {
	log := logx.New("orders", config.String("LOG_LEVEL", "info"))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	pool, err := pg.Open(ctx, "orders", orders.Migrations, orders.MigrationsDir)
	if err != nil {
		return err
	}
	defer pool.Close()

	health := httpx.NewHealth()
	health.Add("postgres", pool.Ping)

	mux := http.NewServeMux()
	health.Mount(mux)
	orders.NewHandler(orders.NewPgStore(pool), orders.NewUsersClient(config.String("USERS_URL", "http://users:8080")), log).Mount(mux)

	return httpx.Serve(ctx, log, health, mux)
}
