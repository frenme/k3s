package scheduler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/scheduler/jobs":
			_, _ = w.Write([]byte(`{"items":[{"name":"heartbeat","schedule":"@every 1m","timeout_seconds":10}]}`))
		case "/internal/v1/scheduler/jobs/heartbeat/run":
			_, _ = w.Write([]byte(`{"job":"heartbeat","status":"succeeded","duration_ms":3}`))
		case "/internal/v1/scheduler/jobs/busy/run":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"code":"job_running","message":"x"}`))
		case "/internal/v1/scheduler/jobs/gone/run":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"job_not_found","message":"x"}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"internal_error","message":"x"}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL)

	jobs, err := c.Jobs(t.Context())
	if err != nil || len(jobs) != 1 || jobs[0].Name != "heartbeat" {
		t.Fatalf("jobs %+v err %v", jobs, err)
	}
	if run, err := c.Run(t.Context(), "heartbeat"); err != nil || run.Status != "succeeded" || run.DurationMS != 3 {
		t.Fatalf("run %+v err %v", run, err)
	}
	if _, err := c.Run(t.Context(), "busy"); !errors.Is(err, ErrJobRunning) {
		t.Fatalf("busy: %v", err)
	}
	if _, err := c.Run(t.Context(), "gone"); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("gone: %v", err)
	}
	if _, err := c.Run(t.Context(), "boom"); err == nil || errors.Is(err, ErrJobNotFound) {
		t.Fatalf("boom: %v", err)
	}
}
