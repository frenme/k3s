package edge

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(c *CORS, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	c.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	return rec
}

func TestCORSPreflightAllowed(t *testing.T) {
	c := NewCORS([]string{"https://example.com/"})
	req := httptest.NewRequest(http.MethodOptions, "/api/users", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := serve(c, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatalf("headers %v", rec.Header())
	}
}

func TestCORSUnknownOrigin(t *testing.T) {
	c := NewCORS([]string{"https://example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Origin", "https://evil.test")
	rec := serve(c, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("status %d headers %v", rec.Code, rec.Header())
	}
}

func TestCORSSimpleRequestHeader(t *testing.T) {
	c := NewCORS([]string{"https://example.com"})
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := serve(c, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "https://example.com" {
		t.Fatalf("headers %v", rec.Header())
	}
}

func TestCORSDisabledPassesThrough(t *testing.T) {
	c := NewCORS(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := serve(c, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("headers %v", rec.Header())
	}
}

func TestCORSWildcard(t *testing.T) {
	c := NewCORS([]string{"*"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any.test")
	if serve(c, req).Header().Get("Access-Control-Allow-Origin") != "https://any.test" {
		t.Fatal("origin not echoed")
	}
}
