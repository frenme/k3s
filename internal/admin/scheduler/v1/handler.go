package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/clients/scheduler"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type jobs interface {
	Jobs(ctx context.Context) ([]scheduler.Job, error)
	Run(ctx context.Context, name string) (scheduler.Run, error)
}

type Handler struct {
	jobs jobs
}

func NewHandler(jobs jobs) *Handler {
	return &Handler{jobs: jobs}
}

func (h *Handler) Register(public huma.API) {
	httpx.Register(public, huma.Operation{
		OperationID: "admin-list-jobs",
		Method:      http.MethodGet,
		Path:        "/v1/admin/scheduler/jobs",
		Summary:     "List scheduled jobs",
		Tags:        []string{"Admin"},
		Errors:      []int{http.StatusBadGateway},
	}, httpx.AccessAdmin, h.list)
	httpx.Register(public, huma.Operation{
		OperationID: "admin-run-job",
		Method:      http.MethodPost,
		Path:        "/v1/admin/scheduler/jobs/{name}/run",
		Summary:     "Run a scheduled job now and wait for the result",
		Tags:        []string{"Admin"},
		Errors:      []int{http.StatusNotFound, http.StatusConflict, http.StatusBadGateway},
	}, httpx.AccessAdmin, h.run)
}

func (h *Handler) list(ctx context.Context, _ *struct{}) (*ListJobsResponse, error) {
	items, err := h.jobs.Jobs(ctx)
	if err != nil {
		return nil, err
	}
	out := &ListJobsResponse{}
	out.Body.Items = make([]Job, 0, len(items))
	for _, j := range items {
		out.Body.Items = append(out.Body.Items, toJob(j))
	}
	return out, nil
}

func (h *Handler) run(ctx context.Context, in *RunJobRequest) (*RunJobResponse, error) {
	run, err := h.jobs.Run(ctx, in.Name)
	if err != nil {
		return nil, err
	}
	return &RunJobResponse{Body: Run{Job: run.Job, Status: run.Status, DurationMS: run.DurationMS, Error: run.Error}}, nil
}
