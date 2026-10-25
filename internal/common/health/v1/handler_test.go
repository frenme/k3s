package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egor/k3s-base/internal/common/health"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

func service(status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/readyz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
	}))
}

func TestHealth(t *testing.T) {
	up, down := service(http.StatusOK), service(http.StatusServiceUnavailable)
	defer up.Close()
	defer down.Close()

	for _, tc := range []struct {
		spec   string
		status int
		want   string
	}{
		{"users=" + up.URL + ",orders=" + up.URL, http.StatusOK, "ok"},
		{"users=" + up.URL + ",orders=" + down.URL, http.StatusServiceUnavailable, "degraded"},
		{"users=" + up.URL + ",gone=http://127.0.0.1:1", http.StatusServiceUnavailable, "degraded"},
	} {
		targets, err := health.ParseTargets(tc.spec)
		if err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		NewHandler(health.NewService(targets)).Register(httpx.NewAPI(mux, "common"))
		srv := httptest.NewServer(mux)
		resp, err := http.Get(srv.URL + "/v1/common/health")
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Status   string    `json:"status"`
			Services []Service `json:"services"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		srv.Close()
		if err != nil || resp.StatusCode != tc.status || body.Status != tc.want || len(body.Services) != 2 {
			t.Fatalf("%s: %d %+v %v", tc.spec, resp.StatusCode, body, err)
		}
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
