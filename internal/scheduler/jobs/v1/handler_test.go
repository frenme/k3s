package v1

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/lock"
	"github.com/egor/k3s-base/internal/scheduler/jobs"
	"github.com/redis/go-redis/v9"
)

func server(t *testing.T) string {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	svc, err := jobs.NewService(slog.New(slog.NewTextHandler(io.Discard, nil)), lock.New(client, "scheduler", "pod"), []jobs.Job{
		{Name: "ok", Schedule: "@every 1h", Timeout: time.Second, Run: func(context.Context) error { return nil }},
		{Name: "bad", Schedule: "@every 1h", Timeout: time.Second, Run: func(context.Context) error { return context.DeadlineExceeded }},
	})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewHandler(svc).Register(httpx.NewInternalAPI(mux, "scheduler"))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func do(t *testing.T, method, url string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestListAndRun(t *testing.T) {
	base := server(t)
	status, body := do(t, http.MethodGet, base+"/internal/v1/scheduler/jobs")
	if items, _ := body["items"].([]any); status != http.StatusOK || len(items) != 2 {
		t.Fatalf("list %d %v", status, body)
	}
	if status, body := do(t, http.MethodPost, base+"/internal/v1/scheduler/jobs/ok/run"); status != http.StatusOK || body["status"] != "succeeded" {
		t.Fatalf("run ok %d %v", status, body)
	}
	if status, body := do(t, http.MethodPost, base+"/internal/v1/scheduler/jobs/bad/run"); status != http.StatusOK || body["status"] != "failed" || body["error"] == "" {
		t.Fatalf("run bad %d %v", status, body)
	}
	if status, body := do(t, http.MethodPost, base+"/internal/v1/scheduler/jobs/nope/run"); status != http.StatusNotFound || body["code"] != "job_not_found" {
		t.Fatalf("run missing %d %v", status, body)
	}
	if status, body := do(t, http.MethodPost, base+"/internal/v1/scheduler/jobs/BAD_NAME/run"); status != http.StatusBadRequest || body["code"] != "invalid_input" {
		t.Fatalf("bad name %d %v", status, body)
	}
}
