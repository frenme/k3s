package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/common/health"
)

type StatusBody struct {
	Status string `json:"status" enum:"ok,unhealthy"`
}

type HealthResponse struct {
	Status int
	Body   StatusBody
}

type Check struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind" enum:"service,infra"`
	Healthy   bool              `json:"healthy"`
	LatencyMS int64             `json:"latency_ms"`
	Error     string            `json:"error,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

type DetailsResponse struct {
	Status int
	Body   struct {
		StatusBody
		CheckedAt time.Time `json:"checked_at"`
		Checks    []Check   `json:"checks"`
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
		Summary:     "Public liveness of every service and infrastructure component, 503 when anything is down",
		Tags:        []string{"Health"},
		Responses: map[string]*huma.Response{
			"503": {Description: "At least one service or infrastructure component is down"},
		},
	}, h.check)
	huma.Register(public, huma.Operation{
		OperationID: "app-health-details",
		Method:      http.MethodGet,
		Path:        "/v1/common/health/details",
		Summary:     "Per service and infrastructure health with error details, admin only at the gateway",
		Tags:        []string{"Health"},
		Responses: map[string]*huma.Response{
			"503": {Description: "At least one service or infrastructure component is down"},
		},
	}, h.details)
}

func (h *Handler) check(ctx context.Context, _ *struct{}) (*HealthResponse, error) {
	report := h.svc.Check(ctx)
	out := &HealthResponse{Status: http.StatusOK, Body: StatusBody{Status: "ok"}}
	if !report.Healthy() {
		out.Status, out.Body.Status = http.StatusServiceUnavailable, "unhealthy"
	}
	return out, nil
}

func (h *Handler) details(ctx context.Context, _ *struct{}) (*DetailsResponse, error) {
	report := h.svc.Check(ctx)
	out := &DetailsResponse{Status: http.StatusOK}
	out.Body.Status = "ok"
	out.Body.CheckedAt = report.CheckedAt
	out.Body.Checks = make([]Check, 0, len(report.Results))
	for _, r := range report.Results {
		out.Body.Checks = append(out.Body.Checks, Check{
			Name: r.Name, Kind: r.Kind, Healthy: r.Healthy, LatencyMS: r.LatencyMS, Error: r.Error, Details: r.Details,
		})
	}
	if !report.Healthy() {
		out.Status, out.Body.Status = http.StatusServiceUnavailable, "unhealthy"
	}
	return out, nil
}
