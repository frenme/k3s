package httpx

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestRequestIDGenerated(t *testing.T) {
	var seen string
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
	}), RequestID())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if seen == "" || rec.Header().Get(RequestIDHeader) != seen {
		t.Fatalf("request id %q header %q", seen, rec.Header().Get(RequestIDHeader))
	}
}

