package main

import (
	"context"
	"strings"
	"time"

	"github.com/egor/k3s-base/internal/gateway"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/config"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

func main() {
	app.Run("gateway", func(ctx context.Context, s *app.Service) error {
		routes, err := gateway.ParseRoutes(config.String("ROUTES", "users=http://users:8080,orders=http://orders:8080"))
		if err != nil {
			return err
		}
		proxy := gateway.NewProxy(routes, s.Log, config.Duration("UPSTREAM_TIMEOUT", 10*time.Second))
		cors := gateway.NewCORS(strings.Split(config.String("CORS_ORIGINS", ""), ","))
		s.Mux.Handle("/api/", httpx.Chain(proxy, cors.Middleware()))
		return nil
	})
}
