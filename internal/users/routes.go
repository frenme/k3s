package users

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/clients/common"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/users/profile"
	profilev1 "github.com/egor/k3s-base/internal/users/profile/v1"
	"github.com/egor/k3s-base/internal/users/storage"
)

const name = "users"

type EnvConfig struct {
	CommonURL string `env:"COMMON_URL" default:"http://common:8081"`
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
	cfg := common.New(env.CommonURL)
	svc := profile.NewService(storage.NewUsers(db, s.Cache()), cfg)
	register(httpx.NewAPI(s.Mux, name), httpx.NewInternalAPI(s.Internal, name), svc, s.Log)
	return nil
}

func OpenAPI() (public, internal *huma.OpenAPI) {
	pub, in := httpx.NewAPI(http.NewServeMux(), name), httpx.NewInternalAPI(http.NewServeMux(), name)
	register(pub, in, nil, nil)
	return pub.OpenAPI(), in.OpenAPI()
}

func register(public, internal huma.API, svc *profile.Service, log *slog.Logger) {
	profilev1.NewHandler(svc, log).Register(public, internal)
}
