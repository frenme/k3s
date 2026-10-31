package users

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/clients/common"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/users/role"
	rolev1 "github.com/egor/k3s-base/internal/users/role/v1"
	"github.com/egor/k3s-base/internal/users/storage"
	"github.com/egor/k3s-base/internal/users/user"
	userv1 "github.com/egor/k3s-base/internal/users/user/v1"
)

const name = "users"

type EnvConfig struct {
	CommonURL string `env:"COMMON_URL" default:"http://common:8081"`
}

func Setup(ctx context.Context, s *app.Service) error {
	env, err := envconfig.Load[EnvConfig]()
	if err != nil {
		return err
	}
	db, err := s.Postgres(ctx, storage.Migrations, storage.MigrationsDir)
	if err != nil {
		return err
	}
	cfg := common.New(env.CommonURL)
	svc := user.NewService(storage.NewUsers(db, s.Cache()), cfg)
	register(httpx.NewAPI(s.Mux, name), httpx.NewInternalAPI(s.Internal, name), svc, role.NewService(storage.NewRoles(db)))
	return nil
}

func OpenAPI() (public, internal *huma.OpenAPI) {
	pub, in := httpx.NewAPI(http.NewServeMux(), name), httpx.NewInternalAPI(http.NewServeMux(), name)
	register(pub, in, nil, nil)
	return pub.OpenAPI(), in.OpenAPI()
}

func register(public, internal huma.API, svc *user.Service, roles *role.Service) {
	userv1.NewHandler(svc).Register(public, internal)
	rolev1.NewHandler(roles).Register(public, internal)
}
