package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
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

	routes, err := gateway.ParseRoutes(config.String("ROUTES", "users=http://users:8080,orders=http://orders:8080"))
	if err != nil {
		log.Error("routes", "err", err)
		os.Exit(1)
	}

	health := httpx.NewHealth()
	var limiter *gateway.Limiter
	if addr := config.String("REDIS_ADDR", ""); addr != "" {
		client := rds.New(addr)
		defer client.Close()
		limiter = gateway.NewLimiter(client, config.Int("RATE_LIMIT", 120), config.Duration("RATE_WINDOW", time.Minute))
	}

	mux := http.NewServeMux()
	health.Mount(mux)
	mux.Handle("/api/", httpx.Chain(gateway.NewProxy(routes, log), limiter.Middleware()))

	handler := httpx.Chain(mux, httpx.RequestID(), httpx.Recover(log), httpx.Observe(log))
	cfg := httpx.DefaultServerConfig(config.String("HTTP_ADDR", ":8080"))
	cfg.DrainDelay = config.Duration("DRAIN_DELAY", cfg.DrainDelay)
	cfg.GraceTimeout = config.Duration("GRACE_TIMEOUT", cfg.GraceTimeout)
	cfg.WriteTimeout = config.Duration("WRITE_TIMEOUT", cfg.WriteTimeout)

	if err := httpx.Run(ctx, cfg, handler, health, log); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}
