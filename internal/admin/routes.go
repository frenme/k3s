package admin

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/admin/data"
	datav1 "github.com/egor/k3s-base/internal/admin/data/v1"
	"github.com/egor/k3s-base/internal/admin/storage"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

const name = "admin"

func Setup(ctx context.Context, s *app.Service) error {
	db, err := s.Postgres(ctx, nil, "")
	if err != nil {
		return err
	}
	svc := data.NewService(storage.NewUsers(db), storage.NewOrders(db))
	register(httpx.NewAPI(s.Mux, name), svc, s.Log)
	return nil
}

func OpenAPI() *huma.OpenAPI {
	api := httpx.NewAPI(http.NewServeMux(), name)
	register(api, nil, nil)
	return api.OpenAPI()
}

func register(public huma.API, svc *data.Service, log *slog.Logger) {
	datav1.NewHandler(svc, log).Register(public)
}
