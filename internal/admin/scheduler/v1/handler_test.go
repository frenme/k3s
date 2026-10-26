package v1

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egor/k3s-base/internal/admin/scheduler"
	clients "github.com/egor/k3s-base/internal/clients/scheduler"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type fakeJobs struct {
	err error
}

func (f fakeJobs) Jobs(context.Context) ([]clients.Job, error) {
	return []clients.Job{{Name: "heartbeat", Schedule: "@every 1m"}}, f.err
}

func (f fakeJobs) Run(context.Context, string) (clients.Run, error) {
	return clients.Run{Job: "heartbeat", Status: "succeeded"}, f.err
}

func call(t *testing.T, jobs fakeJobs, method, path string) (int, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(scheduler.NewService(jobs), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(httpx.NewAPI(mux, "admin"))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	req, _ := http.NewRequest(method, srv.URL+path, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestJobs(t *testing.T) {
	if status, body := call(t, fakeJobs{}, http.MethodGet, "/v1/admin/scheduler/jobs"); status != http.StatusOK || len(body["items"].([]any)) != 1 {
		t.Fatalf("list %d %v", status, body)
	}
	if status, body := call(t, fakeJobs{}, http.MethodPost, "/v1/admin/scheduler/jobs/heartbeat/run"); status != http.StatusOK || body["status"] != "succeeded" {
		t.Fatalf("run %d %v", status, body)
	}
}

func TestJobErrors(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
		code   string
	}{
		"missing": {clients.ErrJobNotFound, http.StatusNotFound, "job_not_found"},
		"running": {clients.ErrJobRunning, http.StatusConflict, "job_running"},
		"down":    {errors.New("connection refused"), http.StatusBadGateway, "scheduler_unavailable"},
	}
	for name, c := range cases {
		status, body := call(t, fakeJobs{err: c.err}, http.MethodPost, "/v1/admin/scheduler/jobs/heartbeat/run")
		if status != c.status || body["code"] != c.code {
			t.Errorf("%s: %d %v", name, status, body)
		}
	}
}
