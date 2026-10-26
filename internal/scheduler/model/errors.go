package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var (
	ErrJobNotFound = httpx.Error{Status: http.StatusNotFound, Code: "job_not_found", Message: "Job not found"}
	ErrJobRunning  = httpx.Error{Status: http.StatusConflict, Code: "job_running", Message: "Job is already running"}
)
