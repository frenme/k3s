package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/scheduler/model"
)

type Job struct {
	Name           string    `json:"name"`
	Schedule       string    `json:"schedule"`
	TimeoutSeconds int64     `json:"timeout_seconds"`
	NextRun        time.Time `json:"next_run"`
}

func toJob(j model.Job) Job {
	return Job{Name: j.Name, Schedule: j.Schedule, TimeoutSeconds: int64(j.Timeout.Seconds()), NextRun: j.NextRun}
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

func toRun(r model.Run) Run {
	status := "succeeded"
	if !r.Succeeded() {
		status = "failed"
	}
	return Run{Job: r.Job, Status: status, DurationMS: r.Duration.Milliseconds(), Error: r.Error}
}
