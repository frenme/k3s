package v1

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/admin/storage"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/pagination"
)

type fakeOrders []storage.Order

func (f fakeOrders) List(_ context.Context, limit int, _ *pagination.Cursor) ([]storage.Order, error) {
	return f[:min(len(f), limit+1)], nil
}

func get(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	orders := fakeOrders{
		{ID: "22222222-2222-4222-8222-222222222222", UserID: "11111111-1111-4111-8111-111111111111", Item: "book", AmountCents: 1, CreatedAt: time.Now()},
		{ID: "33333333-3333-4333-8333-333333333333", UserID: "11111111-1111-4111-8111-111111111111", Item: "pen", AmountCents: 2, CreatedAt: time.Now()},
	}
	NewHandler(orders, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(httpx.NewAPI(mux, "admin"))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestList(t *testing.T) {
	status, body := get(t, "/v1/admin/orders")
	if items, _ := body["items"].([]any); status != http.StatusOK || len(items) != 2 || body["next_cursor"] != nil {
		t.Fatalf("all: %d %v", status, body)
	}
	status, body = get(t, "/v1/admin/orders?limit=1")
	if items, _ := body["items"].([]any); status != http.StatusOK || len(items) != 1 || body["next_cursor"] == "" {
		t.Fatalf("first page: %d %v", status, body)
	}
}

func TestListErrors(t *testing.T) {
	if status, body := get(t, "/v1/admin/orders?cursor=garbage"); status != http.StatusUnprocessableEntity || body["code"] != "invalid_cursor" {
		t.Fatalf("cursor: %d %v", status, body)
	}
}
