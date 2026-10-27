package docs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMergedSpecCoversEveryPublicService(t *testing.T) {
	var spec struct {
		Paths      map[string]any `json:"paths"`
		Components struct {
			Schemas         map[string]any `json:"schemas"`
			SecuritySchemes map[string]any `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(Spec(), &spec); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1/users", "/v1/users/{id}/roles", "/v1/orders", "/v1/admin/users", "/v1/admin/scheduler/jobs", "/v1/common/health", "/v1/common/app-config"} {
		if _, ok := spec.Paths[path]; !ok {
			t.Errorf("%s is missing: run make openapi", path)
		}
	}
	for path := range spec.Paths {
		if strings.Contains(path, "internal") {
			t.Errorf("internal route %s leaked into the public documentation", path)
		}
	}
	if _, ok := spec.Components.SecuritySchemes["bearer"]; !ok || len(spec.Components.Schemas) == 0 {
		t.Fatal("components are incomplete")
	}
}

func TestMount(t *testing.T) {
	mux := http.NewServeMux()
	Mount(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("openapi: %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/docs", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "/api/openapi.json") {
		t.Fatalf("docs: %d", rec.Code)
	}
}
