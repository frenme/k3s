package v1

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/common/config"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Handler struct {
	svc *config.Service
	log *slog.Logger
}

func NewHandler(svc *config.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(public, internal huma.API) {
	huma.Register(public, huma.Operation{
		OperationID: "list-public-config",
		Method:      http.MethodGet,
		Path:        "/v1/common/config",
		Summary:     "Config visible to clients",
		Tags:        []string{"Config"},
	}, h.listPublic)
	huma.Register(internal, huma.Operation{
		OperationID: "internal-get-config",
		Method:      http.MethodGet,
		Path:        "/internal/v1/common/config/{key}",
		Summary:     "Any config entry by key for other services",
		Tags:        []string{"Internal"},
		Errors:      []int{http.StatusNotFound},
	}, h.get)
}

func (h *Handler) listPublic(ctx context.Context, _ *struct{}) (*ListConfigResponse, error) {
	items, err := h.svc.Public(ctx)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	out := &ListConfigResponse{}
	out.Body.Items = make([]Config, 0, len(items))
	for _, s := range items {
		out.Body.Items = append(out.Body.Items, toConfig(s))
	}
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *GetConfigRequest) (*ConfigResponse, error) {
	s, err := h.svc.Get(ctx, in.Key)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	return &ConfigResponse{Body: toConfig(s)}, nil
}
