package orders

import (
	"context"
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
	env, err := envconfig.Load[EnvConfig]()
	if err != nil {
		return err
	}
	db, err := s.Postgres(ctx, storage.Migrations, storage.MigrationsDir)
	if err != nil {
		return err
	}
	usersClient := users.New(env.UsersURL)
	svc := order.NewService(storage.NewOrders(db), usersClient)
	register(httpx.NewAPI(s.Mux, name), svc)
	return nil
}

func OpenAPI() *huma.OpenAPI {
	api := httpx.NewAPI(http.NewServeMux(), name)
	register(api, nil)
	return api.OpenAPI()
}

func register(public huma.API, svc *order.Service) {
	orderv1.NewHandler(svc).Register(public)
}
