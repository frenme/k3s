package gateway

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

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

func TestProxyRoutes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.URL.Path+"?"+r.URL.RawQuery)
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)

	h := NewProxy(map[string]*url.URL{"users": target}, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Second)
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/users/abc?limit=2")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "/users/abc?limit=2" {
		t.Fatalf("body %q", body)
	}

	resp, _ = http.Get(srv.URL + "/api/nothing")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
