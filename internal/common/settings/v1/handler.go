package v1

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/common/settings"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Handler struct {
	svc *settings.Service
	log *slog.Logger
}

func NewHandler(svc *settings.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(public, internal huma.API) {
	huma.Register(public, huma.Operation{
		OperationID: "list-public-settings",
		Method:      http.MethodGet,
		Path:        "/v1/common/settings",
		Summary:     "Settings visible to clients",
		Tags:        []string{"Settings"},
	}, h.listPublic)
	huma.Register(internal, huma.Operation{
		OperationID: "internal-get-setting",
		Method:      http.MethodGet,
		Path:        "/internal/v1/common/settings/{key}",
		Summary:     "Any setting by key for other services",
		Tags:        []string{"Internal"},
		Errors:      []int{http.StatusNotFound},
	}, h.get)
}

func (h *Handler) listPublic(ctx context.Context, _ *struct{}) (*ListSettingsResponse, error) {
	items, err := h.svc.Public(ctx)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	out := &ListSettingsResponse{}
	out.Body.Items = make([]Setting, 0, len(items))
	for _, s := range items {
		out.Body.Items = append(out.Body.Items, toSetting(s))
	}
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *GetSettingRequest) (*SettingResponse, error) {
	s, err := h.svc.Get(ctx, in.Key)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	return &SettingResponse{Body: toSetting(s)}, nil
}
