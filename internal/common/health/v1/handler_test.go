package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egor/k3s-base/internal/common/health"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

func service(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func get(t *testing.T, spec string, infra map[string]httpx.Check, path string, into any) int {
	t.Helper()
	targets, err := health.ParseTargets(spec)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewHandler(health.NewService(targets, infra)).Register(httpx.NewAPI(mux, "common"))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestPublicHealthHidesDetails(t *testing.T) {
	up := service(http.StatusOK, "")
	down := service(http.StatusServiceUnavailable, `{"postgres":"connection refused"}`)
	defer up.Close()
	defer down.Close()
	healthy := map[string]httpx.Check{"redis": func(context.Context) error { return nil }}
	broken := map[string]httpx.Check{"redis": func(context.Context) error { return errors.New("down") }}

	for _, tc := range []struct {
		name   string
		spec   string
		infra  map[string]httpx.Check
		status int
		want   string
	}{
		{"all up", "users=" + up.URL, healthy, http.StatusOK, "ok"},
		{"service down", "users=" + up.URL + ",orders=" + down.URL, healthy, http.StatusServiceUnavailable, "unhealthy"},
		{"service unreachable", "users=" + up.URL + ",gone=http://127.0.0.1:1", healthy, http.StatusServiceUnavailable, "unhealthy"},
		{"infra down", "users=" + up.URL, broken, http.StatusServiceUnavailable, "unhealthy"},
	} {
		var body map[string]any
		status := get(t, tc.spec, tc.infra, "/v1/common/health", &body)
		if status != tc.status || body["status"] != tc.want || len(body) != 1 {
			t.Fatalf("%s: %d %v", tc.name, status, body)
		}
	}
}

func TestDetailsNameEveryFailure(t *testing.T) {
	up := service(http.StatusOK, "")
	down := service(http.StatusServiceUnavailable, `{"postgres":"connection refused"}`)
	defer up.Close()
	defer down.Close()
	infra := map[string]httpx.Check{
		"redis":       func(context.Context) error { return errors.New("dial tcp: i/o timeout") },
		"postgres-rw": func(context.Context) error { return nil },
	}
	var body struct {
		Status string  `json:"status"`
		Checks []Check `json:"checks"`
	}
	status := get(t, "users="+up.URL+",orders="+down.URL, infra, "/v1/common/health/details", &body)
	if status != http.StatusServiceUnavailable || body.Status != "unhealthy" || len(body.Checks) != 4 {
		t.Fatalf("%d %+v", status, body)
	}
	byName := map[string]Check{}
	for _, c := range body.Checks {
		byName[c.Name] = c
	}
	if !byName["users"].Healthy || byName["users"].Kind != health.KindService {
		t.Fatalf("users %+v", byName["users"])
	}
	if c := byName["orders"]; c.Healthy || c.Details["postgres"] != "connection refused" || c.Error == "" {
		t.Fatalf("orders %+v", c)
	}
	if c := byName["redis"]; c.Healthy || c.Kind != health.KindInfra || c.Error != "dial tcp: i/o timeout" {
		t.Fatalf("redis %+v", c)
	}
	if c := byName["postgres-rw"]; !c.Healthy {
		t.Fatalf("postgres-rw %+v", c)
	}
}

func TestParseTargets(t *testing.T) {
	if _, err := health.ParseTargets("users"); err == nil {
		t.Fatal("expected error")
	}
	targets, err := health.ParseTargets("")
	if err != nil || len(targets) != 0 {
		t.Fatalf("%v %v", targets, err)
	}
}
