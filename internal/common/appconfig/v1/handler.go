package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/common/appconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Handler struct {
	svc *appconfig.Service
}

func NewHandler(svc *appconfig.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(public, internal huma.API) {
	httpx.Register(public, huma.Operation{
		OperationID: "list-public-config",
		Method:      http.MethodGet,
		Path:        "/v1/common/app-config",
		Summary:     "AppConfig visible to clients",
		Tags:        []string{"AppConfig"},
	}, httpx.AccessPublic, h.listPublic, httpx.WithCache(10*time.Second))
	httpx.RegisterInternal(internal, huma.Operation{
		OperationID: "internal-get-config",
		Method:      http.MethodGet,
		Path:        "/internal/v1/common/app-config/{key}",
		Summary:     "Any config entry by key for other services",
		Tags:        []string{"Internal"},
		Errors:      []int{http.StatusNotFound},
	}, h.get)
}

func (h *Handler) listPublic(ctx context.Context, _ *struct{}) (*ListAppConfigResponse, error) {
	items, err := h.svc.Public(ctx)
	if err != nil {
		return nil, err
	}
	out := &ListAppConfigResponse{}
	out.Body.Items = make([]AppConfig, 0, len(items))
	for _, s := range items {
		out.Body.Items = append(out.Body.Items, toAppConfig(s))
	}
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *GetAppConfigRequest) (*AppConfigResponse, error) {
	s, err := h.svc.Get(ctx, in.Key)
	if err != nil {
		return nil, err
	}
	return &AppConfigResponse{Body: toAppConfig(s)}, nil
}
