package common

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	commonconfig "github.com/egor/k3s-base/internal/common/config"
	configv1 "github.com/egor/k3s-base/internal/common/config/v1"
	"github.com/egor/k3s-base/internal/common/health"
	healthv1 "github.com/egor/k3s-base/internal/common/health/v1"
	"github.com/egor/k3s-base/internal/common/storage"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/config"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

const name = "common"

func Setup(ctx context.Context, s *app.Service) error {
	db, err := s.Postgres(ctx, storage.Migrations, storage.MigrationsDir)
	if err != nil {
		return err
	}
	targets, err := health.ParseTargets(config.String("HEALTH_TARGETS", ""))
	if err != nil {
		return err
	}
	register(httpx.NewAPI(s.Mux, name), httpx.NewInternalAPI(s.Internal, name),
		commonconfig.NewService(storage.NewConfig(db)), health.NewService(targets), s.Log)
	return nil
}

func OpenAPI() (public, internal *huma.OpenAPI) {
	pub, in := httpx.NewAPI(http.NewServeMux(), name), httpx.NewInternalAPI(http.NewServeMux(), name)
	register(pub, in, nil, nil, nil)
	return pub.OpenAPI(), in.OpenAPI()
}

func register(public, internal huma.API, configSvc *commonconfig.Service, healthSvc *health.Service, log *slog.Logger) {
	configv1.NewHandler(configSvc, log).Register(public, internal)
	healthv1.NewHandler(healthSvc).Register(public)
}
