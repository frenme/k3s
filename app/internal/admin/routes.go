package admin

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	ordersv1 "github.com/egor/k3s-base/internal/admin/orders/v1"
	schedulerv1 "github.com/egor/k3s-base/internal/admin/scheduler/v1"
	"github.com/egor/k3s-base/internal/admin/storage"
	usersv1 "github.com/egor/k3s-base/internal/admin/users/v1"
	"github.com/egor/k3s-base/internal/clients/scheduler"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

const name = "admin"

type EnvConfig struct {
	SchedulerURL string `env:"SCHEDULER_URL" default:"http://scheduler:8081"`
}

func Setup(ctx context.Context, s *app.Service) error {
	env, err := envconfig.Load[EnvConfig]()
	if err != nil {
		return err
	}
	db, err := s.PostgresNoMigrations(ctx)
	if err != nil {
		return err
	}
	register(httpx.NewAPI(s.Mux, name), storage.NewUsers(db), storage.NewOrders(db), scheduler.New(env.SchedulerURL))
	return nil
}

func OpenAPI() *huma.OpenAPI {
	api := httpx.NewAPI(http.NewServeMux(), name)
	register(api, nil, nil, nil)
	return api.OpenAPI()
}

func register(public huma.API, users *storage.Users, orders *storage.Orders, jobs *scheduler.Client) {
	usersv1.NewHandler(users).Register(public)
	ordersv1.NewHandler(orders).Register(public)
	schedulerv1.NewHandler(jobs).Register(public)
}
