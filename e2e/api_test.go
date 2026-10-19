//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

var (
	base   = envOr("BASE_URL", "http://localhost:8080")
	host   = os.Getenv("HOST_HEADER")
	origin = envOr("CORS_ORIGIN", "http://localhost:5173")
	client = &http.Client{Timeout: 10 * time.Second}
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
	req, err := http.NewRequest(method, base+path, &payload)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if host != "" {
		req.Host = host
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out := response{status: resp.StatusCode, header: resp.Header}
	_ = json.NewDecoder(resp.Body).Decode(&out.body)
	return out
}

func expect(t *testing.T, r response, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("expected %d, got %d: %v", status, r.status, r.body)
	}
}

func TestUnknownRoute(t *testing.T) {
	expect(t, call(t, http.MethodGet, "/api/unknown", nil, nil), http.StatusNotFound)
}

func TestCORSPreflight(t *testing.T) {
	r := call(t, http.MethodOptions, "/api/users", nil, map[string]string{
		"Origin":                        origin,
		"Access-Control-Request-Method": "POST",
	})
	if got := r.header.Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("allow origin %q", got)
	}
}

func TestUsersAndOrders(t *testing.T) {
	email := fmt.Sprintf("e2e-%d@example.com", time.Now().UnixNano())

	created := call(t, http.MethodPost, "/api/users", map[string]string{"email": email, "name": "E2E"}, nil)
	expect(t, created, http.StatusCreated)
	userID, _ := created.body["id"].(string)
	if userID == "" {
		t.Fatal("user id missing")
	}

	expect(t, call(t, http.MethodPost, "/api/users", map[string]string{"email": email, "name": "E2E"}, nil), http.StatusConflict)
	expect(t, call(t, http.MethodGet, "/api/users/"+userID, nil, nil), http.StatusOK)

	order := map[string]any{"user_id": userID, "item": "widget", "amount_cents": 1999}
	key := map[string]string{"Idempotency-Key": "e2e-" + userID}

	first := call(t, http.MethodPost, "/api/orders", order, key)
	expect(t, first, http.StatusCreated)
	orderID, _ := first.body["id"].(string)

	expect(t, call(t, http.MethodGet, "/api/orders/"+orderID, nil, nil), http.StatusOK)
	expect(t, call(t, http.MethodGet, "/api/orders/00000000-0000-0000-0000-000000000000", nil, nil), http.StatusNotFound)

	replay := call(t, http.MethodPost, "/api/orders", order, key)
	expect(t, replay, http.StatusOK)
	if replay.body["id"] != orderID {
		t.Fatalf("replay returned another order: %v", replay.body["id"])
	}

	unknown := map[string]any{"user_id": "00000000-0000-0000-0000-000000000000", "item": "x", "amount_cents": 1}
	expect(t, call(t, http.MethodPost, "/api/orders", unknown, nil), http.StatusUnprocessableEntity)

	list := call(t, http.MethodGet, "/api/orders?user_id="+userID, nil, nil)
	expect(t, list, http.StatusOK)
	if items, _ := list.body["items"].([]any); len(items) != 1 {
		t.Fatalf("expected 1 order, got %d", len(items))
	}
}
