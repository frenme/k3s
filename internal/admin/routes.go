package admin

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/admin/data"
	datav1 "github.com/egor/k3s-base/internal/admin/data/v1"
	"github.com/egor/k3s-base/internal/admin/scheduler"
	schedulerv1 "github.com/egor/k3s-base/internal/admin/scheduler/v1"
	"github.com/egor/k3s-base/internal/admin/storage"
	clients "github.com/egor/k3s-base/internal/clients/scheduler"
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
	register(httpx.NewAPI(s.Mux, name),
		data.NewService(storage.NewUsers(db), storage.NewOrders(db)),
		scheduler.NewService(clients.New(env.SchedulerURL)), s.Log)
	return nil
}

func OpenAPI() *huma.OpenAPI {
	api := httpx.NewAPI(http.NewServeMux(), name)
	register(api, nil, nil, nil)
	return api.OpenAPI()
}

func register(public huma.API, dataSvc *data.Service, schedulerSvc *scheduler.Service, log *slog.Logger) {
	datav1.NewHandler(dataSvc, log).Register(public)
	schedulerv1.NewHandler(schedulerSvc, log).Register(public)
}
