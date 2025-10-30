package gateway

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
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

