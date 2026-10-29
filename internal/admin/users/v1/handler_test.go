package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/admin/storage"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/pagination"
)

type fakeUsers []storage.User

func (f fakeUsers) List(_ context.Context, limit int, _ *pagination.Cursor) ([]storage.User, error) {
	return f[:min(len(f), limit+1)], nil
}

func get(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	users := fakeUsers{
		{ID: "11111111-1111-4111-8111-111111111111", Email: "a@b.co", Name: "Ann", CreatedAt: time.Now()},
		{ID: "22222222-2222-4222-8222-222222222222", Email: "b@b.co", Name: "Bob", CreatedAt: time.Now()},
		{ID: "33333333-3333-4333-8333-333333333333", Email: "c@b.co", Name: "Cat", CreatedAt: time.Now()},
	}
	NewHandler(users).Register(httpx.NewAPI(mux, "admin"))
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
	status, body := get(t, "/v1/admin/users")
	if items, _ := body["items"].([]any); status != http.StatusOK || len(items) != 3 || body["next_cursor"] != nil {
		t.Fatalf("all: %d %v", status, body)
	}
	status, body = get(t, "/v1/admin/users?limit=2")
	if items, _ := body["items"].([]any); status != http.StatusOK || len(items) != 2 || body["next_cursor"] == "" {
		t.Fatalf("first page: %d %v", status, body)
	}
}

func TestListErrors(t *testing.T) {
	if status, body := get(t, "/v1/admin/users?limit=0"); status != http.StatusBadRequest || body["code"] != "invalid_input" {
		t.Fatalf("limit=0: %d %v", status, body)
	}
	if status, body := get(t, "/v1/admin/users?cursor=garbage"); status != http.StatusBadRequest || body["code"] != "invalid_cursor" {
		t.Fatalf("cursor: %d %v", status, body)
	}
}
