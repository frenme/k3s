package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/scheduler/jobs"
	jobsv1 "github.com/egor/k3s-base/internal/scheduler/jobs/v1"
)

const (
	name        = "scheduler"
	stopTimeout = 15 * time.Second
)

func Setup(ctx context.Context, s *app.Service) error {
	locker := s.Locker()
	if locker == nil {
		return errors.New("REDIS_ADDR is required: job locks live in redis")
	}
	client := s.Redis()
	s.Health.Add("redis", func(ctx context.Context) error { return client.Ping(ctx).Err() })

	svc, err := jobs.NewService(s.Log, locker, jobs.Catalog(s.Log))
	if err != nil {
		return err
	}
	svc.Start()
	s.OnClose(func() {
		select {
		case <-svc.Stop():
		case <-time.After(stopTimeout):
		}
	})
	register(httpx.NewInternalAPI(s.Internal, name), svc, s.Log)
	return nil
}

func OpenAPI() *huma.OpenAPI {
	api := httpx.NewInternalAPI(http.NewServeMux(), name)
	register(api, nil, nil)
	return api.OpenAPI()
}

func register(internal huma.API, svc *jobs.Service, log *slog.Logger) {
	jobsv1.NewHandler(svc, log).Register(internal)
}
