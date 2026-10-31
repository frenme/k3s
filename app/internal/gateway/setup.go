package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/gateway/auth"
	"github.com/egor/k3s-base/internal/gateway/cache"
	"github.com/egor/k3s-base/internal/gateway/docs"
	"github.com/egor/k3s-base/internal/gateway/edge"
	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/gateway/proxy"
	"github.com/egor/k3s-base/internal/gateway/validate"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type EnvConfig struct {
	Routes            string        `env:"ROUTES" required:"true"`
	UsersURL          string        `env:"USERS_URL" default:"http://users:8081"`
	CORSOrigins       []string      `env:"CORS_ORIGINS"`
	TrustedProxies    []string      `env:"TRUSTED_PROXIES" default:"10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8,fc00::/7,::1/128"`
	UpstreamTimeout   time.Duration `env:"UPSTREAM_TIMEOUT" default:"10s"`
	UpstreamFailures  int           `env:"UPSTREAM_FAILURE_THRESHOLD" default:"5"`
	UpstreamCooldown  time.Duration `env:"UPSTREAM_COOLDOWN" default:"10s"`
	UpstreamMaxActive int           `env:"UPSTREAM_MAX_CONCURRENT" default:"200"`
	MaxBody           int64         `env:"MAX_BODY_BYTES" default:"1048576"`
	RequestValidation bool          `env:"REQUEST_VALIDATION" default:"true"`
	RolesCacheTTL     time.Duration `env:"ROLES_CACHE_TTL" default:"30s"`
	CacheEntries      int           `env:"RESPONSE_CACHE_ENTRIES" default:"500"`
	JWT               auth.JWTConfig
}

type userRoles struct {
	client *users.Client
}

func (u userRoles) Roles(ctx context.Context, id string) ([]string, error) {
	roles, err := u.client.Roles(ctx, id)
	if errors.Is(err, users.ErrNotFound) {
		return nil, nil
	}
	return roles, err
}

func Setup(ctx context.Context, s *app.Service) error {
	env, err := envconfig.Load[EnvConfig]()
	if err != nil {
		return err
	}

	routes, err := proxy.ParseRoutes(env.Routes)
	if err != nil {
		return err
	}
	verifier, err := auth.NewVerifier(env.JWT, s.Log)
	if err != nil {
		return err
	}
	verifier.Start(ctx)
	clientIP, err := edge.NewClientIP(env.TrustedProxies)
	if err != nil {
		return err
	}
	validation := func(next http.Handler) http.Handler { return next }
	if env.RequestValidation {
		validator, err := validate.New(docs.Spec())
		if err != nil {
			return err
		}
		validation = validator.Middleware()
	}

	roles := auth.NewCachedRoles(userRoles{users.New(env.UsersURL)}, env.RolesCacheTTL, 10000)
	handler := httpx.Chain(
		proxy.New(proxy.Options{
			Routes:         routes,
			Log:            s.Log,
			DefaultTimeout: env.UpstreamTimeout,
			Guard:          proxy.GuardConfig{FailureThreshold: env.UpstreamFailures, Cooldown: env.UpstreamCooldown, MaxConcurrent: env.UpstreamMaxActive},
		}),
		clientIP.Middleware(),
		edge.NewCORS(env.CORSOrigins).Middleware(),
		policy.Resolve(policy.Default()),
		auth.Authorize(verifier, roles, s.Log),
		edge.BodyLimit(env.MaxBody),
		validation,
		cache.New(env.CacheEntries).Middleware(),
	)
	docs.Mount(s.Mux)
	s.Mux.Handle("/api/", handler)
	return nil
}
