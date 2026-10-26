package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/clients/scheduler"
)

type Job struct {
	Name           string    `json:"name"`
	Schedule       string    `json:"schedule"`
	TimeoutSeconds int64     `json:"timeout_seconds"`
	NextRun        time.Time `json:"next_run"`
}

func toJob(j scheduler.Job) Job {
	return Job{Name: j.Name, Schedule: j.Schedule, TimeoutSeconds: j.TimeoutSeconds, NextRun: j.NextRun}
}

type ListJobsResponse struct {
	Body struct {
		Items []Job `json:"items"`
	}
}

type RunJobRequest struct {
	Name string `path:"name" pattern:"^[a-z0-9-]+$" maxLength:"100"`
}

type Run struct {
	Job        string `json:"job"`
	Status     string `json:"status" enum:"succeeded,failed"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

type RunJobResponse struct {
	Body Run
}
