package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/egor/k3s-base/internal/clients/scheduler"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

var (
	ErrJobNotFound = httpx.Error{Status: http.StatusNotFound, Code: "job_not_found", Message: "Job not found"}
	ErrJobRunning  = httpx.Error{Status: http.StatusConflict, Code: "job_running", Message: "Job is already running"}
	ErrUnavailable = httpx.Error{Status: http.StatusBadGateway, Code: "scheduler_unavailable", Message: "Scheduler is temporarily unavailable"}
)

type jobs interface {
	Jobs(ctx context.Context) ([]scheduler.Job, error)
	Run(ctx context.Context, name string) (scheduler.Run, error)
}

type Service struct {
	jobs jobs
}

func NewService(jobs jobs) *Service {
	return &Service{jobs: jobs}
}

func (s *Service) Jobs(ctx context.Context) ([]scheduler.Job, error) {
	items, err := s.jobs.Jobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return items, nil
}

func (s *Service) Run(ctx context.Context, name string) (scheduler.Run, error) {
	run, err := s.jobs.Run(ctx, name)
	switch {
	case errors.Is(err, scheduler.ErrJobNotFound):
		return scheduler.Run{}, ErrJobNotFound
	case errors.Is(err, scheduler.ErrJobRunning):
		return scheduler.Run{}, ErrJobRunning
	case err != nil:
		return scheduler.Run{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return run, nil
}
