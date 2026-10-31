package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/scheduler/jobs"
)

type Handler struct {
	svc *jobs.Service
}

func NewHandler(svc *jobs.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(internal huma.API) {
	httpx.RegisterInternal(internal, huma.Operation{
		OperationID: "internal-list-jobs",
		Method:      http.MethodGet,
		Path:        "/internal/v1/scheduler/jobs",
		Summary:     "List scheduled jobs",
		Tags:        []string{"Internal"},
	}, h.list)
	httpx.RegisterInternal(internal, huma.Operation{
		OperationID: "internal-run-job",
		Method:      http.MethodPost,
		Path:        "/internal/v1/scheduler/jobs/{name}/run",
		Summary:     "Run a job now and wait for the result",
		Tags:        []string{"Internal"},
		Errors:      []int{http.StatusNotFound, http.StatusConflict},
	}, h.run)
}

func (h *Handler) list(_ context.Context, _ *struct{}) (*ListJobsResponse, error) {
	items := h.svc.List()
	out := &ListJobsResponse{}
	out.Body.Items = make([]Job, 0, len(items))
	for _, j := range items {
		out.Body.Items = append(out.Body.Items, toJob(j))
	}
	return out, nil
}

func (h *Handler) run(ctx context.Context, in *RunJobRequest) (*RunJobResponse, error) {
	run, err := h.svc.Run(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	return &RunJobResponse{Body: toRun(run)}, nil
}
