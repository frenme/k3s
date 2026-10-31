package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newProbesServer(h *Probes) *httptest.Server {
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
	srv := newProbesServer(NewProbes())
	defer srv.Close()
	if got := status(t, srv.URL+"/healthz"); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
}

func TestReadyzFailingCheck(t *testing.T) {
	h := NewProbes()
	h.Add("db", func(ctx context.Context) error { return errors.New("down") })
	srv := newProbesServer(h)
	defer srv.Close()
	if got := status(t, srv.URL+"/readyz"); got != http.StatusServiceUnavailable {
		t.Fatalf("status %d", got)
	}
}

func TestReadyzDraining(t *testing.T) {
	h := NewProbes()
	srv := newProbesServer(h)
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
	srv := newProbesServer(NewProbes())
	defer srv.Close()
	if got := status(t, srv.URL+"/version"); got != http.StatusOK {
		t.Fatalf("status %d", got)
	}
}
