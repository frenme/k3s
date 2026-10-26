package gateway

import (
	"context"
	"time"

	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type EnvConfig struct {
	Routes          string        `env:"ROUTES" required:"true"`
	CORSOrigins     []string      `env:"CORS_ORIGINS"`
	UpstreamTimeout time.Duration `env:"UPSTREAM_TIMEOUT" default:"10s"`
	JWT             JWTConfig
}

func Setup(ctx context.Context, s *app.Service) error {
	var env EnvConfig
	if err := envconfig.Load(&env); err != nil {
		return err
	}
	s.Public = true
	routes, err := ParseRoutes(env.Routes)
	if err != nil {
		return err
	}
	verifier, err := NewVerifier(env.JWT)
	if err != nil {
		return err
	}
	proxy := NewProxy(routes, s.Log, env.UpstreamTimeout)
	cors := NewCORS(env.CORSOrigins)
	s.Mux.Handle("/api/", httpx.Chain(proxy, cors.Middleware(), Authorize(DefaultRules, verifier)))
	return nil
}
