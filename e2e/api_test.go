//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/golang-jwt/jwt/v5"
)

var (
	specs  = loadSpecs()
	base   = envOr("BASE_URL", "http://localhost:8080")
	host   = os.Getenv("HOST_HEADER")
	origin = envOr("CORS_ORIGIN", "http://localhost:5173")
	client = &http.Client{Timeout: 10 * time.Second}

	jwtSecret = envOr("E2E_JWT_SECRET", "dev-jwt-secret")
)

func loadSpecs() []routers.Router {
	var out []routers.Router
	for _, file := range []string{"../api/users.yaml", "../api/orders.yaml", "../api/admin.yaml", "../api/common.yaml"} {
		doc, err := openapi3.NewLoader().LoadFromFile(file)
		if err != nil {
			panic(err)
		}
		if err := doc.Validate(context.Background()); err != nil {
			panic(fmt.Sprintf("%s: %v", file, err))
		}
		router, err := gorillamux.NewRouter(doc)
		if err != nil {
			panic(err)
		}
		out = append(out, router)
	}
	return out
}

func validate(t *testing.T, req *http.Request, status int, header http.Header, body []byte) {
	t.Helper()
	var (
		route  *routers.Route
		params map[string]string
		err    error
	)
	for _, spec := range specs {
		if route, params, err = spec.FindRoute(req); err == nil {
			break
		}
	}
	if route == nil {
		return
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
		Status:                 status,
		Header:                 header,
		Options:                &openapi3filter.Options{IncludeResponseStatus: true},
	}
	input.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(req.Context(), input); err != nil {
		t.Fatalf("%s %s %d does not match the spec in api/: %v", req.Method, req.URL.Path, status, err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

const (
	noAuth     = "X-E2E-No-Auth"
	noValidate = "X-E2E-No-Validate"
)

const seededAdminID = "00000000-0000-4000-8000-000000000001"

func userToken(t *testing.T) string {
	return tokenFor(t, "e2e-user")
}

func adminAuth(t *testing.T) map[string]string {
	return as(t, seededAdminID)
}

func as(t *testing.T, subject string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + tokenFor(t, subject)}
}

func tokenFor(t *testing.T, subject string) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte(jwtSecret))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

type response struct {
	status int
	header http.Header
	body   map[string]any
}

func call(t *testing.T, method, path string, body any, headers map[string]string) response {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequestWithContext(t.Context(), method, base+path, &payload)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if host != "" {
		req.Host = host
	}
	if _, skip := headers[noAuth]; !skip {
		req.Header.Set("Authorization", "Bearer "+userToken(t))
	}
	for k, v := range headers {
		if k != noAuth && k != noValidate {
			req.Header.Set(k, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if _, skip := headers[noValidate]; !skip {
		validate(t, req, resp.StatusCode, resp.Header, raw)
	}
	out := response{status: resp.StatusCode, header: resp.Header}
	_ = json.Unmarshal(raw, &out.body)
	return out
}

func expect(t *testing.T, r response, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("expected %d, got %d: %v", status, r.status, r.body)
	}
}

func TestUnknownRoute(t *testing.T) {
	cases := []struct {
		method, path string
		status       int
	}{
		{http.MethodGet, "/api/unknown", http.StatusNotFound},
		{http.MethodGet, "/api/v1/users/1/nope", http.StatusNotFound},
		{http.MethodGet, "/api/v1/users/internal/v1/users/11111111-1111-4111-8111-111111111111", http.StatusNotFound},
		{http.MethodDelete, "/api/v1/orders", http.StatusMethodNotAllowed},
	}
	for _, c := range cases {
		r := call(t, c.method, c.path, nil, nil)
		expect(t, r, c.status)
		if code, _ := r.body["code"].(string); code == "" || r.body["message"] == "" {
			t.Fatalf("%s %s: no json error in %v", c.method, c.path, r.body)
		}
	}
}

func TestDocsThroughGateway(t *testing.T) {
	for _, path := range []string{"/api/v1/users/docs", "/api/v1/users/openapi.json", "/api/v1/orders/docs", "/api/v1/orders/openapi.json"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if host != "" {
			req.Host = host
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
	}
}

func TestCommon(t *testing.T) {
	config := call(t, http.MethodGet, "/api/v1/common/app-config", nil, nil)
	expect(t, config, http.StatusOK)
	items, _ := config.body["items"].([]any)
	found := false
	for _, item := range items {
		if m, _ := item.(map[string]any); m["key"] == "registration_enabled" {
			found = true
		}
	}
	if !found {
		t.Fatalf("registration_enabled missing in %v", config.body)
	}
	health := call(t, http.MethodGet, "/api/v1/common/health", nil, map[string]string{noAuth: ""})
	expect(t, health, http.StatusOK)
	if health.body["status"] != "ok" || len(health.body) != 1 {
		t.Fatalf("health %v", health.body)
	}
	expect(t, call(t, http.MethodGet, "/api/v1/common/health/details", nil, map[string]string{noAuth: ""}), http.StatusUnauthorized)
	expect(t, call(t, http.MethodGet, "/api/v1/common/health/details", nil, nil), http.StatusForbidden)
	details := call(t, http.MethodGet, "/api/v1/common/health/details", nil, adminAuth(t))
	expect(t, details, http.StatusOK)
	if checks, _ := details.body["checks"].([]any); len(checks) == 0 {
		t.Fatalf("details %v", details.body)
	}
}

func TestAuthentication(t *testing.T) {
	for _, path := range []string{"/api/v1/orders", "/api/v1/users/00000000-0000-0000-0000-000000000000"} {
		denied := call(t, http.MethodGet, path, nil, map[string]string{noAuth: ""})
		expect(t, denied, http.StatusUnauthorized)
		if denied.body["code"] != "unauthorized" {
			t.Fatalf("%s: body %v", path, denied.body)
		}
	}
	expect(t, call(t, http.MethodGet, "/api/v1/orders", nil, map[string]string{"Authorization": "Bearer nope"}), http.StatusUnauthorized)
}

func TestAdminNeedsTheAdminRole(t *testing.T) {
	for _, path := range []string{"/api/v1/admin/users", "/api/v1/admin/orders", "/api/v1/users?limit=1"} {
		expect(t, call(t, http.MethodGet, path, nil, map[string]string{noAuth: ""}), http.StatusUnauthorized)
		denied := call(t, http.MethodGet, path, nil, nil)
		expect(t, denied, http.StatusForbidden)
		if denied.body["code"] != "forbidden" {
			t.Fatalf("%s: body %v", path, denied.body)
		}
		expect(t, call(t, http.MethodGet, path, nil, adminAuth(t)), http.StatusOK)
	}
}

func TestGrantingTheAdminRole(t *testing.T) {
	created := call(t, http.MethodPost, "/api/v1/users", map[string]string{"email": fmt.Sprintf("role-%d@example.com", time.Now().UnixNano()), "name": "Role"}, nil)
	expect(t, created, http.StatusCreated)
	id, _ := created.body["id"].(string)

	roles := call(t, http.MethodGet, "/api/v1/users/"+id+"/roles", nil, adminAuth(t))
	expect(t, roles, http.StatusOK)
	if got, _ := roles.body["roles"].([]any); len(got) != 1 || got[0] != "user" {
		t.Fatalf("a new user must only have the user role: %v", roles.body)
	}
	expect(t, call(t, http.MethodPut, "/api/v1/users/"+id+"/roles/admin", nil, adminAuth(t)), http.StatusOK)

	promoted := map[string]string{"Authorization": "Bearer " + tokenFor(t, id)}
	expect(t, call(t, http.MethodGet, "/api/v1/admin/users", nil, promoted), http.StatusOK)
	expect(t, call(t, http.MethodPut, "/api/v1/users/"+id+"/roles/root", nil, adminAuth(t)), http.StatusUnprocessableEntity)
}

func TestRequestsAreValidatedAtTheGateway(t *testing.T) {
	r := call(t, http.MethodPost, "/api/v1/users", map[string]string{"email": "nope", "name": "x"}, nil)
	expect(t, r, http.StatusUnprocessableEntity)
	if details, _ := r.body["details"].([]any); r.body["code"] != "invalid_input" || len(details) == 0 {
		t.Fatalf("body %v", r.body)
	}
	expect(t, call(t, http.MethodGet, "/api/v1/orders?limit=0", nil, nil), http.StatusUnprocessableEntity)
}

func TestBodyLimitAndRouteTimeoutSettings(t *testing.T) {
	huge := map[string]any{"item": strings.Repeat("x", 70000), "amount_cents": 1}
	expect(t, call(t, http.MethodPost, "/api/v1/orders", huge, map[string]string{noValidate: ""}), http.StatusRequestEntityTooLarge)
}

func TestPublicConfigIsCachedAtTheGateway(t *testing.T) {
	call(t, http.MethodGet, "/api/v1/common/app-config", nil, nil)
	second := call(t, http.MethodGet, "/api/v1/common/app-config", nil, nil)
	expect(t, second, http.StatusOK)
	if got := second.header.Get("X-Cache"); got != "HIT" {
		t.Fatalf("X-Cache %q", got)
	}
}

func TestAggregatedDocumentation(t *testing.T) {
	for _, path := range []string{"/api/docs", "/api/openapi.json"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if host != "" {
			req.Host = host
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
	}
}

func TestRequestIDIsFreshTraceID(t *testing.T) {
	r := call(t, http.MethodGet, "/api/v1/orders?limit=1", nil, map[string]string{
		"traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
	})
	expect(t, r, http.StatusOK)
	ids := r.header.Values("X-Request-ID")
	if len(ids) != 1 || !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(ids[0]) {
		t.Fatalf("X-Request-ID %v", ids)
	}
	if ids[0] == "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatal("gateway continued a client supplied trace")
	}
}

func TestCORSPreflight(t *testing.T) {
	r := call(t, http.MethodOptions, "/api/v1/users", nil, map[string]string{
		"Origin":                        origin,
		"Access-Control-Request-Method": "POST",
	})
	if got := r.header.Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("allow origin %q", got)
	}
}

func TestUsersAndOrders(t *testing.T) {
	email := fmt.Sprintf("e2e-%d@example.com", time.Now().UnixNano())

	created := call(t, http.MethodPost, "/api/v1/users", map[string]string{"email": email, "name": "E2E"}, nil)
	expect(t, created, http.StatusCreated)
	userID, _ := created.body["id"].(string)
	if userID == "" {
		t.Fatal("user id missing")
	}
	me := as(t, userID)
	stranger := as(t, "00000000-0000-4000-8000-0000000000ff")

	expect(t, call(t, http.MethodPost, "/api/v1/users", map[string]string{"email": email, "name": "E2E"}, nil), http.StatusConflict)
	expect(t, call(t, http.MethodGet, "/api/v1/users/"+userID, nil, me), http.StatusOK)
	expect(t, call(t, http.MethodGet, "/api/v1/users/"+userID, nil, stranger), http.StatusNotFound)
	expect(t, call(t, http.MethodGet, "/api/v1/users/"+userID, nil, nil), http.StatusUnauthorized)

	order := map[string]any{"item": "widget", "amount_cents": 1999}
	key := map[string]string{"Idempotency-Key": "e2e-" + userID, "Authorization": me["Authorization"]}

	first := call(t, http.MethodPost, "/api/v1/orders", order, key)
	expect(t, first, http.StatusCreated)
	orderID, _ := first.body["id"].(string)
	if first.body["user_id"] != userID {
		t.Fatalf("the order must belong to the token subject: %v", first.body)
	}

	expect(t, call(t, http.MethodGet, "/api/v1/orders/"+orderID, nil, me), http.StatusOK)
	expect(t, call(t, http.MethodGet, "/api/v1/orders/"+orderID, nil, stranger), http.StatusNotFound)
	expect(t, call(t, http.MethodGet, "/api/v1/orders/00000000-0000-0000-0000-000000000000", nil, me), http.StatusNotFound)

	replay := call(t, http.MethodPost, "/api/v1/orders", order, key)
	expect(t, replay, http.StatusOK)
	if replay.body["id"] != orderID {
		t.Fatalf("replay returned another order: %v", replay.body["id"])
	}

	changed := map[string]any{"item": "widget", "amount_cents": 1}
	reused := call(t, http.MethodPost, "/api/v1/orders", changed, key)
	expect(t, reused, http.StatusUnprocessableEntity)
	if reused.body["code"] != "idempotency_key_reused" {
		t.Fatalf("reused key: %v", reused.body)
	}

	expect(t, call(t, http.MethodPost, "/api/v1/orders", map[string]any{"item": "x", "amount_cents": 1}, stranger), http.StatusUnprocessableEntity)

	list := call(t, http.MethodGet, "/api/v1/orders", nil, me)
	expect(t, list, http.StatusOK)
	if items, _ := list.body["items"].([]any); len(items) != 1 {
		t.Fatalf("expected 1 order, got %d", len(items))
	}
	others := call(t, http.MethodGet, "/api/v1/orders", nil, stranger)
	expect(t, others, http.StatusOK)
	if items, _ := others.body["items"].([]any); len(items) != 0 {
		t.Fatalf("another user sees %d orders", len(items))
	}
}
