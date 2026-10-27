package policy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

func TestRuleMatching(t *testing.T) {
	cases := []struct {
		rule   Rule
		method string
		path   string
		want   bool
	}{
		{Rule{Method: "GET", Path: "/api/v1/users"}, "GET", "/api/v1/users", true},
		{Rule{Method: "GET", Path: "/api/v1/users"}, "POST", "/api/v1/users", false},
		{Rule{Method: "GET", Path: "/api/v1/users"}, "GET", "/api/v1/users/1", false},
		{Rule{Path: "/api/v1/admin/**"}, "DELETE", "/api/v1/admin/a/b/c", true},
		{Rule{Path: "/api/v1/admin/**"}, "GET", "/api/v1/administrators", false},
		{Rule{Method: "GET", Path: "/api/v1/*/docs"}, "GET", "/api/v1/orders/docs", true},
		{Rule{Method: "GET", Path: "/api/v1/*/docs"}, "GET", "/api/v1/orders/docs/x", false},
	}
	for _, c := range cases {
		if got := c.rule.matches(c.method, split(c.path)); got != c.want {
			t.Errorf("%+v %s %s: got %v", c.rule, c.method, c.path, got)
		}
	}
}

func TestFind(t *testing.T) {
	table := Default()
	cases := []struct {
		method, path string
		access       Access
		template     string
		matched      bool
	}{
		{"GET", "/api/v1/common/health", AccessPublic, "GET /api/v1/common/health", true},
		{"GET", "/api/v1/common/health/details", AccessAdmin, "GET /api/v1/common/health/details", true},
		{"POST", "/api/v1/users", AccessPublic, "POST /api/v1/users", true},
		{"GET", "/api/v1/users", AccessAdmin, "GET /api/v1/users", true},
		{"GET", "/api/v1/users/abc", AccessUser, "GET /api/v1/users/*", true},
		{"GET", "/api/v1/users/abc/roles", AccessAdmin, "GET /api/v1/users/*/roles", true},
		{"GET", "/api/v1/orders/docs", AccessPublic, "GET /api/v1/*/docs", true},
		{"GET", "/api/v1/admin/anything/new", AccessAdmin, "/api/v1/admin/**", true},
		{"GET", "/api/v1/nothing", AccessUser, "unmatched", false},
	}
	for _, c := range cases {
		got := table.Find(c.method, c.path)
		if got.Rule.Access != c.access || got.Template != c.template || got.Matched != c.matched {
			t.Errorf("%s %s: %+v", c.method, c.path, got)
		}
	}
}

func TestGeneratedRulesCarryRouteSettings(t *testing.T) {
	var found bool
	for _, r := range generated {
		if r.Method == http.MethodPost && r.Path == "/api/v1/orders" {
			found = true
			if r.Timeout != 5*time.Second || r.MaxBody != 65536 {
				t.Fatalf("settings %+v", r)
			}
		}
		if r.Method == http.MethodGet && r.Path == "/api/v1/common/app-config" && r.CacheTTL != 10*time.Second {
			t.Fatalf("cache %+v", r)
		}
	}
	if !found {
		t.Fatal("run make openapi: orders route missing from the generated rules")
	}
}

func TestResolveSetsRouteForHandlersAndMetrics(t *testing.T) {
	var got Route
	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = From(r.Context()) }), Resolve(Default()))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/orders/42", nil))
	if got.Template != "GET /api/v1/orders/*" || got.Rule.Access != AccessUser {
		t.Fatalf("%+v", got)
	}
	if From(httptest.NewRequest(http.MethodGet, "/", nil).Context()).Template != "unmatched" {
		t.Fatal("missing route must read as unmatched")
	}
}
