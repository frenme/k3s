package edge

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveClientIP(t *testing.T) {
	c, err := NewClientIP([]string{"10.0.0.0/8", "127.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, remote, xff, want string
	}{
		{"direct client ignores a forged header", "203.0.113.9:4000", "1.2.3.4", "203.0.113.9"},
		{"trusted proxy passes the client through", "10.1.2.3:4000", "198.51.100.7", "198.51.100.7"},
		{"forged left entry is skipped", "10.1.2.3:4000", "6.6.6.6, 198.51.100.7", "198.51.100.7"},
		{"chain of trusted proxies", "10.1.2.3:4000", "198.51.100.7, 10.9.9.9", "198.51.100.7"},
		{"trusted proxy without header", "10.1.2.3:4000", "", "10.1.2.3"},
		{"garbage header falls back to the peer", "10.1.2.3:4000", "not-an-ip", "10.1.2.3"},
		{"ipv6 mapped peer", "[::ffff:10.1.2.3]:4000", "198.51.100.7", "198.51.100.7"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = tc.remote
		if tc.xff != "" {
			req.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := c.Resolve(req); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestMiddlewareExposesTheIP(t *testing.T) {
	c, _ := NewClientIP([]string{"10.0.0.0/8"})
	var got string
	h := c.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = IPFrom(r.Context()) }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.5:1"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got != "198.51.100.7" {
		t.Fatalf("got %q", got)
	}
}

func TestNewClientIPRejectsBadCIDR(t *testing.T) {
	if _, err := NewClientIP([]string{"nope"}); err == nil {
		t.Fatal("expected error")
	}
}
