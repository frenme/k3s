package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newHealthServer(h *Health) *httptest.Server {
	mux := http.NewServeMux()
	h.Mount(mux)
	return httptest.NewServer(mux)
}

func status(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode
}

func TestHealthz(t *testing.T) {
	srv := newHealthServer(NewHealth())
	defer srv.Close()
	if got := status(t, srv.URL+"/healthz"); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
}

func TestReadyzFailingCheck(t *testing.T) {
	h := NewHealth()
	h.Add("db", func(ctx context.Context) error { return errors.New("down") })
	srv := newHealthServer(h)
	defer srv.Close()
	if got := status(t, srv.URL+"/readyz"); got != http.StatusServiceUnavailable {
		t.Fatalf("status %d", got)
	}
}

func TestReadyzDraining(t *testing.T) {
	h := NewHealth()
	srv := newHealthServer(h)
	defer srv.Close()
	if got := status(t, srv.URL+"/readyz"); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
	h.SetReady(false)
	if got := status(t, srv.URL+"/readyz"); got != http.StatusServiceUnavailable {
		t.Fatalf("status %d", got)
	}
}

func TestVersion(t *testing.T) {
	srv := newHealthServer(NewHealth())
	defer srv.Close()
	if got := status(t, srv.URL+"/version"); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
}
