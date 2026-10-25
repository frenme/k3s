package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/common/health"
)

type Service struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
}

type HealthResponse struct {
	Status int
	Body   struct {
		Status   string    `json:"status" enum:"ok,degraded"`
		Services []Service `json:"services"`
	}
}

type Handler struct {
	svc *health.Service
}

func NewHandler(svc *health.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(public huma.API) {
	huma.Register(public, huma.Operation{
		OperationID: "app-health",
		Method:      http.MethodGet,
		Path:        "/v1/common/health",
		Summary:     "Health of every service, 503 when any of them is not ready",
		Tags:        []string{"Health"},
		Responses: map[string]*huma.Response{
			"503": {Description: "At least one service is not ready"},
		},
	}, h.check)
}

func (h *Handler) check(ctx context.Context, _ *struct{}) (*HealthResponse, error) {
	out := &HealthResponse{Status: http.StatusOK}
	out.Body.Status = "ok"
	for _, s := range h.svc.Check(ctx) {
		out.Body.Services = append(out.Body.Services, Service{Name: s.Name, Healthy: s.Healthy})
		if !s.Healthy {
			out.Status = http.StatusServiceUnavailable
			out.Body.Status = "degraded"
		}
	}
	if out.Body.Services == nil {
		out.Body.Services = []Service{}
	}
	return out, nil
}
