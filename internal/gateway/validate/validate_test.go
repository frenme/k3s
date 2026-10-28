package validate

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egor/k3s-base/internal/gateway/docs"
	"github.com/egor/k3s-base/internal/gateway/edge"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

func serve(t *testing.T, method, target, body string) (int, map[string]any, string) {
	t.Helper()
	v, err := New(docs.Spec())
	if err != nil {
		t.Fatal(err)
	}
	var downstream string
	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		downstream = string(raw)
		w.WriteHeader(http.StatusNoContent)
	}), v.Middleware())
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out, downstream
}

func TestValidRequestsPassWithTheirBodyIntact(t *testing.T) {
	body := `{"email":"ann@example.com","name":"Ann"}`
	code, _, downstream := serve(t, http.MethodPost, "/api/v1/users", body)
	if code != http.StatusNoContent || downstream != body {
		t.Fatalf("status %d, downstream body %q", code, downstream)
	}
	if code, _, _ := serve(t, http.MethodGet, "/api/v1/orders?limit=10", ""); code != http.StatusNoContent {
		t.Fatalf("list: %d", code)
	}
}

func TestInvalidRequestsAreRejectedBeforeTheService(t *testing.T) {
	cases := []struct {
		name, method, target, body string
		status                     int
		code                       string
	}{
		{"bad email", "POST", "/api/v1/users", `{"email":"nope","name":"Ann"}`, 422, "invalid_input"},
		{"missing field", "POST", "/api/v1/users", `{"email":"a@b.co"}`, 422, "invalid_input"},
		{"unknown field", "POST", "/api/v1/users", `{"email":"a@b.co","name":"Ann","admin":true}`, 422, "invalid_input"},
		{"malformed json", "POST", "/api/v1/users", `{bad`, 400, "bad_request"},
		{"limit below minimum", "GET", "/api/v1/orders?limit=0", "", 422, "invalid_input"},
		{"limit above maximum", "GET", "/api/v1/orders?limit=100000", "", 422, "invalid_input"},
		{"owner in the body", "POST", "/api/v1/orders", `{"user_id":"11111111-1111-4111-8111-111111111111","item":"x","amount_cents":1}`, 422, "invalid_input"},
		{"negative amount", "POST", "/api/v1/orders", `{"item":"x","amount_cents":-1}`, 422, "invalid_input"},
	}
	for _, c := range cases {
		code, body, downstream := serve(t, c.method, c.target, c.body)
		if code != c.status || body["code"] != c.code || downstream != "" {
			t.Errorf("%s: status %d body %v reached the service: %v", c.name, code, body, downstream != "")
		}
		if c.status == 422 {
			if details, _ := body["details"].([]any); len(details) == 0 {
				t.Errorf("%s: no details in %v", c.name, body)
			}
		}
	}
}

func TestBodyOverTheLimitIsTooLargeEvenWithoutContentLength(t *testing.T) {
	v, err := New(docs.Spec())
	if err != nil {
		t.Fatal(err)
	}
	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), edge.BodyLimit(100), v.Middleware())
	body := `{"email":"ann@example.com","name":"` + strings.Repeat("a", 500) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(body))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "payload_too_large") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
}

func TestUnknownRoutesAreLeftToTheProxy(t *testing.T) {
	if code, _, _ := serve(t, http.MethodGet, "/api/v1/nothing/at/all", ""); code != http.StatusNoContent {
		t.Fatalf("status %d", code)
	}
}

func TestNewRejectsBrokenSpecs(t *testing.T) {
	if _, err := New([]byte(`{`)); err == nil {
		t.Fatal("expected an error")
	}
}
