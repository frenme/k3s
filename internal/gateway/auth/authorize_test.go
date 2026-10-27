package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/golang-jwt/jwt/v5"
)

type fakeRoles struct {
	roles map[string][]string
	err   error
	calls atomic.Int32
}

func (f *fakeRoles) Roles(_ context.Context, id string) ([]string, error) {
	f.calls.Add(1)
	return f.roles[id], f.err
}

func authHandler(t *testing.T, roles RoleSource, got *http.Header) http.Handler {
	t.Helper()
	v, err := NewVerifier(JWTConfig{Secret: testSecret}, discard())
	if err != nil {
		t.Fatal(err)
	}
	return httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = r.Header.Clone()
	}), policy.Resolve(policy.Default()), Authorize(v, roles, discard()))
}

func bearer(t *testing.T, sub string) string {
	t.Helper()
	claims := validClaims()
	claims.Subject = sub
	return "Bearer " + sign(t, jwt.SigningMethodHS256, []byte(testSecret), claims)
}

func TestAuthorize(t *testing.T) {
	roles := &fakeRoles{roles: map[string][]string{"admin-1": {"admin", "user"}, "user-1": {"user"}}}
	var seen http.Header
	h := authHandler(t, roles, &seen)

	cases := []struct {
		name, method, path, auth string
		want                     int
		scheme                   string
	}{
		{name: "public health", method: "GET", path: "/api/v1/common/health", want: 200},
		{name: "public registration", method: "POST", path: "/api/v1/users", want: 200},
		{name: "docs", method: "GET", path: "/api/v1/orders/docs", want: 200},
		{name: "orders need a token", method: "GET", path: "/api/v1/orders", want: 401, scheme: "Bearer"},
		{name: "unknown path needs a token", method: "GET", path: "/api/v1/nothing", want: 401, scheme: "Bearer"},
		{name: "garbage token", method: "GET", path: "/api/v1/orders", auth: "Bearer nope", want: 401, scheme: "Bearer"},
		{name: "basic is not a token", method: "GET", path: "/api/v1/orders", auth: "Basic YWRtaW46YWRtaW4=", want: 401, scheme: "Bearer"},
		{name: "user token", method: "GET", path: "/api/v1/orders", auth: bearer(t, "user-1"), want: 200},
		{name: "lowercase scheme", method: "GET", path: "/api/v1/orders", auth: "bearer " + bearer(t, "user-1")[7:], want: 200},
		{name: "user cannot list users", method: "GET", path: "/api/v1/users", auth: bearer(t, "user-1"), want: 403},
		{name: "admin lists users", method: "GET", path: "/api/v1/users", auth: bearer(t, "admin-1"), want: 200},
		{name: "admin catch-all", method: "GET", path: "/api/v1/admin/anything", auth: bearer(t, "user-1"), want: 403},
		{name: "admin without a token", method: "GET", path: "/api/v1/admin/users", want: 401, scheme: "Bearer"},
		{name: "unknown subject is not an admin", method: "GET", path: "/api/v1/admin/users", auth: bearer(t, "ghost"), want: 403},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.path, nil)
		if c.auth != "" {
			req.Header.Set("Authorization", c.auth)
		}
		seen = nil
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.want)
			continue
		}
		if c.want == 401 && rec.Header().Get("WWW-Authenticate")[:len(c.scheme)] != c.scheme {
			t.Errorf("%s: WWW-Authenticate %q", c.name, rec.Header().Get("WWW-Authenticate"))
		}
		if c.want == 200 && seen.Get("Authorization") != "" {
			t.Errorf("%s: credentials leaked upstream", c.name)
		}
	}
}

func TestAuthorizeSetsIdentityHeaders(t *testing.T) {
	roles := &fakeRoles{roles: map[string][]string{"admin-1": {"admin", "user"}}}
	var seen http.Header
	h := authHandler(t, roles, &seen)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", bearer(t, "admin-1"))
	req.Header.Set(UserIDHeader, "attacker")
	req.Header.Set(UserRolesHeader, "root")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen.Get(UserIDHeader) != "admin-1" || seen.Get(UserRolesHeader) != "admin,user" {
		t.Fatalf("headers %v", seen)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/common/health", nil)
	req.Header.Set(UserIDHeader, "attacker")
	req.Header.Set(UserRolesHeader, "root")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen.Get(UserIDHeader) != "" || seen.Get(UserRolesHeader) != "" {
		t.Fatalf("spoofed headers reached a public route: %v", seen)
	}
}

func TestAdminRoutesFailClosedWhenRolesAreUnavailable(t *testing.T) {
	var seen http.Header
	h := authHandler(t, &fakeRoles{err: errors.New("users is down")}, &seen)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", bearer(t, "admin-1"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || seen != nil {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestUserRoutesDoNotLookUpRoles(t *testing.T) {
	roles := &fakeRoles{}
	var seen http.Header
	h := authHandler(t, roles, &seen)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	req.Header.Set("Authorization", bearer(t, "user-1"))
	h.ServeHTTP(httptest.NewRecorder(), req)
	if roles.calls.Load() != 0 {
		t.Fatal("roles were looked up for a route that needs no role")
	}
}

func TestCachedRoles(t *testing.T) {
	src := &fakeRoles{roles: map[string][]string{"u": {"admin"}}}
	now := time.Now()
	c := NewCachedRoles(src, time.Minute, 10)
	c.now = func() time.Time { return now }

	for range 3 {
		if got, err := c.Roles(t.Context(), "u"); err != nil || len(got) != 1 {
			t.Fatalf("%v %v", got, err)
		}
	}
	if src.calls.Load() != 1 {
		t.Fatalf("source called %d times", src.calls.Load())
	}
	now = now.Add(2 * time.Minute)
	_, _ = c.Roles(t.Context(), "u")
	if src.calls.Load() != 2 {
		t.Fatalf("entry did not expire: %d calls", src.calls.Load())
	}

	failing := &fakeRoles{err: errors.New("down")}
	cf := NewCachedRoles(failing, time.Minute, 10)
	_, _ = cf.Roles(t.Context(), "u")
	_, _ = cf.Roles(t.Context(), "u")
	if failing.calls.Load() != 2 {
		t.Fatal("errors must not be cached")
	}
}

func TestCachedRolesIsBounded(t *testing.T) {
	src := &fakeRoles{roles: map[string][]string{}}
	c := NewCachedRoles(src, time.Hour, 3)
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		_, _ = c.Roles(t.Context(), id)
	}
	if len(c.entries) > 3 {
		t.Fatalf("%d entries", len(c.entries))
	}
}
