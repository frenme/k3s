package scheduler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

func TestClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/scheduler/jobs":
			_, _ = w.Write([]byte(`{"items":[{"name":"heartbeat","schedule":"@every 1m","timeout_seconds":10}]}`))
		case "/internal/v1/scheduler/jobs/heartbeat/run":
			_, _ = w.Write([]byte(`{"job":"heartbeat","status":"succeeded","duration_ms":3}`))
		default:
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"code":"job_running","message":"Job is already running"}`))
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
	if _, err := c.Run(t.Context(), "busy"); !errors.Is(err, httpx.Error{Code: "job_running"}) {
		t.Fatalf("busy: %v", err)
	}
}
