package common

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/common/appconfig"
	appconfigv1 "github.com/egor/k3s-base/internal/common/appconfig/v1"
	"github.com/egor/k3s-base/internal/common/health"
	healthv1 "github.com/egor/k3s-base/internal/common/health/v1"
	"github.com/egor/k3s-base/internal/common/storage"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/pg"
)

const name = "common"

type EnvConfig struct {
	HealthTargets     string        `env:"HEALTH_TARGETS"`
	AppConfigCacheTTL time.Duration `env:"APP_CONFIG_CACHE_TTL" default:"30s"`
}

func Setup(ctx context.Context, s *app.Service) error {
	var env EnvConfig
	if err := envconfig.Load(&env); err != nil {
		return err
	}
	db, err := s.Postgres(ctx, storage.Migrations, storage.MigrationsDir)
	if err != nil {
		return err
	}
	targets, err := health.ParseTargets(env.HealthTargets)
	if err != nil {
		return err
	}
	register(httpx.NewAPI(s.Mux, name), httpx.NewInternalAPI(s.Internal, name),
		appconfig.NewService(storage.NewAppConfig(db, s.Cache(), env.AppConfigCacheTTL)),
		health.NewService(targets, infraProbes(s, db)...), s.Log)
	return nil
}

func infraProbes(s *app.Service, db *pg.DB) []health.Probe {
	probes := []health.Probe{{Name: "postgres-rw", Run: db.PingWrite}}
	if db.HasReplica() {
		probes = append(probes, health.Probe{Name: "postgres-ro", Run: db.PingRead})
	}
	if client := s.Redis(); client != nil {
		probes = append(probes, health.Probe{Name: "redis", Run: func(ctx context.Context) error { return client.Ping(ctx).Err() }})
	}
	return probes
}

func OpenAPI() (public, internal *huma.OpenAPI) {
	pub, in := httpx.NewAPI(http.NewServeMux(), name), httpx.NewInternalAPI(http.NewServeMux(), name)
	register(pub, in, nil, nil, nil)
	return pub.OpenAPI(), in.OpenAPI()
}

func register(public, internal huma.API, appConfigSvc *appconfig.Service, healthSvc *health.Service, log *slog.Logger) {
	appconfigv1.NewHandler(appConfigSvc, log).Register(public, internal)
	healthv1.NewHandler(healthSvc).Register(public)
}
