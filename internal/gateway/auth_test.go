package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/golang-jwt/jwt/v5"
)

func authHandler(t *testing.T, got *http.Header) http.Handler {
	t.Helper()
	v, err := NewVerifier(JWTConfig{Secret: testSecret})
	if err != nil {
		t.Fatal(err)
	}
	return httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = r.Header.Clone()
	}), Authorize(DefaultRules, v))
}

func TestAuthorize(t *testing.T) {
	var seen http.Header
	h := authHandler(t, &seen)
	token := sign(t, jwt.SigningMethodHS256, []byte(testSecret), validClaims())

	cases := []struct {
		name, method, path string
		header             map[string]string
		basic              [2]string
		want               int
		wantScheme         string
	}{
		{name: "public health", method: "GET", path: "/api/v1/common/health", want: 200},
		{name: "public config", method: "GET", path: "/api/v1/common/app-config", want: 200},
		{name: "public registration", method: "POST", path: "/api/v1/users", want: 200},
		{name: "docs", method: "GET", path: "/api/v1/orders/docs", want: 200},
		{name: "openapi", method: "GET", path: "/api/v1/users/openapi.json", want: 200},
		{name: "orders need token", method: "GET", path: "/api/v1/orders", want: 401, wantScheme: "Bearer"},
		{name: "user by id needs token", method: "GET", path: "/api/v1/users/abc", want: 401, wantScheme: "Bearer"},
		{name: "unknown path needs token", method: "GET", path: "/api/v1/nothing", want: 401, wantScheme: "Bearer"},
		{name: "valid token", method: "GET", path: "/api/v1/orders", header: map[string]string{"Authorization": "Bearer " + token}, want: 200},
		{name: "lowercase scheme", method: "GET", path: "/api/v1/orders", header: map[string]string{"Authorization": "bearer " + token}, want: 200},
		{name: "bad token", method: "GET", path: "/api/v1/orders", header: map[string]string{"Authorization": "Bearer nope"}, want: 401, wantScheme: "Bearer"},
		{name: "basic is not a token", method: "GET", path: "/api/v1/orders", basic: [2]string{adminUser, adminPassword}, want: 401, wantScheme: "Bearer"},
		{name: "user list is admin only", method: "GET", path: "/api/v1/users", header: map[string]string{"Authorization": "Bearer " + token}, want: 401, wantScheme: "Basic"},
		{name: "admin without password", method: "GET", path: "/api/v1/admin/users", want: 401, wantScheme: "Basic"},
		{name: "token is not admin", method: "GET", path: "/api/v1/admin/users", header: map[string]string{"Authorization": "Bearer " + token}, want: 401, wantScheme: "Basic"},
		{name: "wrong admin password", method: "GET", path: "/api/v1/admin/users", basic: [2]string{adminUser, "wrong"}, want: 401, wantScheme: "Basic"},
		{name: "admin", method: "GET", path: "/api/v1/admin/users", basic: [2]string{adminUser, adminPassword}, want: 200},
		{name: "health details need admin", method: "GET", path: "/api/v1/common/health/details", want: 401, wantScheme: "Basic"},
		{name: "health details as admin", method: "GET", path: "/api/v1/common/health/details", basic: [2]string{adminUser, adminPassword}, want: 200},
		{name: "admin root", method: "GET", path: "/api/v1/admin", want: 401, wantScheme: "Basic"},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		for k, v := range c.header {
			req.Header.Set(k, v)
		}
		if c.basic[0] != "" {
			req.SetBasicAuth(c.basic[0], c.basic[1])
		}
		seen = nil
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.want)
			continue
		}
		if c.want == 401 {
			if got := rec.Header().Get("WWW-Authenticate"); len(got) < len(c.wantScheme) || got[:len(c.wantScheme)] != c.wantScheme {
				t.Errorf("%s: WWW-Authenticate %q", c.name, got)
			}
			continue
		}
		if seen.Get("Authorization") != "" {
			t.Errorf("%s: credentials leaked upstream", c.name)
		}
	}
}

func TestAuthorizeSetsAndSpoofsUserID(t *testing.T) {
	var seen http.Header
	h := authHandler(t, &seen)
	token := sign(t, jwt.SigningMethodHS256, []byte(testSecret), validClaims())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(UserIDHeader, "attacker")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got := seen.Get(UserIDHeader); got != "user-1" {
		t.Fatalf("user id %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/common/health", nil)
	req.Header.Set(UserIDHeader, "attacker")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got := seen.Get(UserIDHeader); got != "" {
		t.Fatalf("spoofed header reached a public route: %q", got)
	}
}

func TestRuleMatching(t *testing.T) {
	cases := []struct {
		rule     Rule
		method   string
		path     string
		expected bool
	}{
		{Rule{"GET", "/api/v1/users", AccessAdmin}, "GET", "/api/v1/users", true},
		{Rule{"GET", "/api/v1/users", AccessAdmin}, "POST", "/api/v1/users", false},
		{Rule{"GET", "/api/v1/users", AccessAdmin}, "GET", "/api/v1/users/1", false},
		{Rule{"", "/api/v1/admin/**", AccessAdmin}, "DELETE", "/api/v1/admin/a/b/c", true},
		{Rule{"", "/api/v1/admin/**", AccessAdmin}, "GET", "/api/v1/administrators", false},
		{Rule{"GET", "/api/v1/*/docs", AccessPublic}, "GET", "/api/v1/orders/docs", true},
		{Rule{"GET", "/api/v1/*/docs", AccessPublic}, "GET", "/api/v1/orders/docs/x", false},
	}
	for _, c := range cases {
		segments := splitPath(c.path)
		if got := c.rule.matches(c.method, segments); got != c.expected {
			t.Errorf("%+v %s %s: got %v", c.rule, c.method, c.path, got)
		}
	}
}

func TestGeneratedRulesCoverTheAPI(t *testing.T) {
	if len(generatedRules) == 0 {
		t.Fatal("run make openapi: no rules were generated from the specs")
	}
	want := map[string]Access{
		"GET /api/v1/common/health":         AccessPublic,
		"GET /api/v1/common/health/details": AccessAdmin,
		"POST /api/v1/users":                AccessPublic,
		"GET /api/v1/users":                 AccessAdmin,
		"GET /api/v1/orders/*":              AccessUser,
		"GET /api/v1/admin/users":           AccessAdmin,
	}
	got := map[string]Access{}
	for _, r := range generatedRules {
		got[r.Method+" "+r.Path] = r.Access
	}
	for route, access := range want {
		if got[route] != access {
			t.Errorf("%s: access %v, want %v", route, got[route], access)
		}
	}
}
