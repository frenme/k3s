package edge

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

func serveWithLimit(t *testing.T, defaultMax int64, route policy.Route, body string, contentLength int64) (int, error) {
	t.Helper()
	var readErr error
	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}), BodyLimit(defaultMax))
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.ContentLength = contentLength
	req = req.WithContext(withRoute(req.Context(), route))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, readErr
}

func withRoute(ctx context.Context, route policy.Route) context.Context {
	var out context.Context
	policy.Resolve(policy.Table{route.Rule})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { out = r.Context() })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, route.Rule.Path, nil).WithContext(ctx))
	return out
}

func TestBodyLimit(t *testing.T) {
	route := policy.Route{Rule: policy.Rule{Method: http.MethodPost, Path: "/x", MaxBody: 10}}

	if code, err := serveWithLimit(t, 1000, route, "short", 5); code != http.StatusOK || err != nil {
		t.Fatalf("small body: %d %v", code, err)
	}
	if code, _ := serveWithLimit(t, 1000, route, strings.Repeat("a", 50), 50); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared length: %d", code)
	}
	if _, err := serveWithLimit(t, 1000, route, strings.Repeat("a", 50), -1); err == nil {
		t.Fatal("a chunked body over the limit must fail while reading")
	}
	open := policy.Route{Rule: policy.Rule{Method: http.MethodPost, Path: "/x"}}
	if code, _ := serveWithLimit(t, 20, open, strings.Repeat("a", 50), 50); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("default limit: %d", code)
	}
}
