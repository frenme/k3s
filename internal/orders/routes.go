package orders

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/orders/order"
	orderv1 "github.com/egor/k3s-base/internal/orders/order/v1"
	"github.com/egor/k3s-base/internal/orders/storage"
)

const name = "orders"

type EnvConfig struct {
	UsersURL string `env:"USERS_URL" default:"http://users:8081"`
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
	usersClient := users.New(env.UsersURL)
	svc := order.NewService(storage.NewOrders(db), usersClient)
	register(httpx.NewAPI(s.Mux, name), svc, s.Log)
	return nil
}

func OpenAPI() *huma.OpenAPI {
	api := httpx.NewAPI(http.NewServeMux(), name)
	register(api, nil, nil)
	return api.OpenAPI()
}

func register(public huma.API, svc *order.Service, log *slog.Logger) {
	orderv1.NewHandler(svc, log).Register(public)
}
