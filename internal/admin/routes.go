package admin

import (
	"context"
	"log/slog"
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
	var env EnvConfig
	if err := envconfig.Load(&env); err != nil {
		return err
	}
	db, err := s.Postgres(ctx, nil, "")
	if err != nil {
		return err
	}
	register(httpx.NewAPI(s.Mux, name), storage.NewUsers(db), storage.NewOrders(db), scheduler.New(env.SchedulerURL), s.Log)
	return nil
}

func OpenAPI() *huma.OpenAPI {
	api := httpx.NewAPI(http.NewServeMux(), name)
	register(api, nil, nil, nil, nil)
	return api.OpenAPI()
}

func register(public huma.API, users *storage.Users, orders *storage.Orders, jobs *scheduler.Client, log *slog.Logger) {
	usersv1.NewHandler(users, log).Register(public)
	ordersv1.NewHandler(orders, log).Register(public)
	schedulerv1.NewHandler(jobs, log).Register(public)
}
