package proxy

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/gateway/edge"
	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func gatewayFor(t *testing.T, upstream http.Handler, table policy.Table, guard GuardConfig, timeout time.Duration, trusted ...string) *httptest.Server {
	t.Helper()
	up := httptest.NewServer(upstream)
	t.Cleanup(up.Close)
	target, _ := url.Parse(up.URL)
	if len(trusted) == 0 {
		trusted = []string{"127.0.0.0/8"}
	}
	clientIP, _ := edge.NewClientIP(trusted)
	h := httpx.Chain(
		New(Options{Routes: map[string]*url.URL{"users": target}, Log: discard(), DefaultTimeout: timeout, Guard: guard}),
		clientIP.Middleware(), policy.Resolve(table),
	)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

var defaultGuard = GuardConfig{FailureThreshold: 3, Cooldown: time.Hour, MaxConcurrent: 100}

func TestParseRoutes(t *testing.T) {
	routes, err := ParseRoutes("users=http://users:8080, orders=http://orders:8080")
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes["orders"].Host != "orders:8080" {
		t.Fatalf("unexpected routes %v", routes)
	}
	for _, bad := range []string{"", "users", "users=", "=http://x", "users=notaurl"} {
		if _, err := ParseRoutes(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestProxyRoutesAndHidesInternals(t *testing.T) {
	srv := gatewayFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(httpx.RequestIDHeader, "upstream")
		_, _ = io.WriteString(w, r.URL.Path+"?"+r.URL.RawQuery+" for="+r.Header.Get("X-Forwarded-For"))
	}), policy.Default(), defaultGuard, time.Second)

	resp, err := http.Get(srv.URL + "/api/v1/users/abc?limit=2")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.HasPrefix(string(body), "/v1/users/abc?limit=2") || !strings.Contains(string(body), "for=127.0.0.1") {
		t.Fatalf("body %q", body)
	}
	if ids := resp.Header.Values(httpx.RequestIDHeader); len(ids) != 0 {
		t.Fatalf("upstream request id leaked: %v", ids)
	}

	for _, path := range []string{"/api/nothing", "/api/users/abc", "/api/x1/users", "/api/v1/nothing", "/api/v1/users/internal", "/api/v1/users/internal/v1/users/1", "/api/v1/users/a/internal/b"} {
		resp, _ = http.Get(srv.URL + path)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
	}
}

func TestClientSuppliedForwardingHeadersAreReplaced(t *testing.T) {
	var got http.Header
	srv := gatewayFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.Header.Clone() }), policy.Default(), defaultGuard, time.Second, "10.0.0.0/8")
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/users/1", nil)
	req.Header.Set("X-Forwarded-For", "6.6.6.6")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got.Get("X-Forwarded-For") != "127.0.0.1" {
		t.Fatalf("X-Forwarded-For %q", got.Get("X-Forwarded-For"))
	}
}

func TestRouteTimeoutBecomesGatewayTimeout(t *testing.T) {
	table := policy.Table{{Method: "GET", Path: "/api/v1/users/slow", Access: policy.AccessPublic, Timeout: 50 * time.Millisecond}}
	srv := gatewayFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}), table, defaultGuard, 10*time.Second)

	start := time.Now()
	resp, err := http.Get(srv.URL + "/api/v1/users/slow")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusGatewayTimeout || time.Since(start) > time.Second {
		t.Fatalf("status %d after %s", resp.StatusCode, time.Since(start))
	}
}

func TestCircuitOpensAfterRepeatedFailures(t *testing.T) {
	var hits atomic.Int32
	srv := gatewayFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}), policy.Default(), defaultGuard, time.Second)

	for range 3 {
		resp, _ := http.Get(srv.URL + "/api/v1/users/1")
		_ = resp.Body.Close()
	}
	before := hits.Load()
	resp, err := http.Get(srv.URL + "/api/v1/users/1")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" || hits.Load() != before {
		t.Fatalf("status %d, upstream hit again: %v", resp.StatusCode, hits.Load() != before)
	}
}

func TestOverloadedServiceIsRejectedFast(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	srv := gatewayFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
	}), policy.Default(), GuardConfig{FailureThreshold: 5, Cooldown: time.Hour, MaxConcurrent: 1}, 5*time.Second)

	done := make(chan struct{})
	go func() {
		resp, err := http.Get(srv.URL + "/api/v1/users/1")
		if err == nil {
			_ = resp.Body.Close()
		}
		close(done)
	}()
	<-entered
	resp, err := http.Get(srv.URL + "/api/v1/users/2")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
	close(release)
	<-done
}

func TestGuardHalfOpenLetsOneProbeThrough(t *testing.T) {
	healthy := false
	next := roundTripFunc(func(*http.Request) (*http.Response, error) {
		if healthy {
			return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
		}
		return nil, errors.New("down")
	})
	g := newGuard("x", next, GuardConfig{FailureThreshold: 2, Cooldown: time.Minute, MaxConcurrent: 5})
	now := time.Now()
	g.now = func() time.Time { return now }
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	for range 2 {
		_, _ = g.RoundTrip(req)
	}
	if _, err := g.RoundTrip(req); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected an open circuit: %v", err)
	}
	now = now.Add(2 * time.Minute)
	healthy = true
	if _, err := g.RoundTrip(req); err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if _, err := g.RoundTrip(req); err != nil {
		t.Fatalf("circuit stayed open after a successful probe: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
