package users

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/users/profile"
	profilev1 "github.com/egor/k3s-base/internal/users/profile/v1"
	"github.com/egor/k3s-base/internal/users/storage"
)

const name = "users"

func Setup(ctx context.Context, s *app.Service) error {
	pools, err := s.Postgres(ctx, storage.Migrations, storage.MigrationsDir)
	if err != nil {
		return err
	}
	svc := profile.NewService(storage.NewUsers(pools, s.Cache()))
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
