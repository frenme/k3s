package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/egor/k3s-base/internal/gateway"
	"github.com/egor/k3s-base/internal/platform/config"
	"github.com/egor/k3s-base/internal/platform/httpx"
	"github.com/egor/k3s-base/internal/platform/logx"
	"github.com/egor/k3s-base/internal/platform/rds"
)

func main() {
	log := logx.New("gateway", config.String("LOG_LEVEL", "info"))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	routes, err := gateway.ParseRoutes(config.String("ROUTES", "users=http://users:8080,orders=http://orders:8080"))
	if err != nil {
		return err
	}

	var limiter *gateway.Limiter
	if addr := config.String("REDIS_ADDR", ""); addr != "" {
		client := rds.New(addr)
		defer client.Close()
		limiter = gateway.NewLimiter(client,
			config.Int("RATE_LIMIT", 120),
			config.Duration("RATE_WINDOW", time.Minute),
			config.Int("TRUSTED_PROXY_HOPS", 1))
	}

	health := httpx.NewHealth()
	proxy := gateway.NewProxy(routes, log, config.Duration("UPSTREAM_TIMEOUT", 10*time.Second))

	mux := http.NewServeMux()
	health.Mount(mux)
	cors := gateway.NewCORS(strings.Split(config.String("CORS_ORIGINS", ""), ","))
	mux.Handle("/api/", httpx.Chain(proxy, cors.Middleware(), limiter.Middleware()))

	return httpx.Serve(ctx, log, health, mux)
}
