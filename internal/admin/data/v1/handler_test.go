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

	"github.com/egor/k3s-base/internal/admin/data"
	"github.com/egor/k3s-base/internal/admin/model"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type fakeUsers []model.User

func (f fakeUsers) List(ctx context.Context, limit int) ([]model.User, error) { return f, nil }

type fakeOrders []model.Order

func (f fakeOrders) List(ctx context.Context, limit int) ([]model.Order, error) { return f, nil }

func TestLists(t *testing.T) {
	mux := http.NewServeMux()
	svc := data.NewService(
		fakeUsers{{ID: "11111111-1111-4111-8111-111111111111", Email: "a@b.co", Name: "Ann", CreatedAt: time.Now()}},
		fakeOrders{{ID: "22222222-2222-4222-8222-222222222222", UserID: "11111111-1111-4111-8111-111111111111", Item: "book", AmountCents: 1}},
	)
	NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(httpx.NewAPI(mux, "admin"))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, path := range []string{"/v1/admin/users", "/v1/admin/orders"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Items []map[string]any `json:"items"`
		}
		err = json.NewDecoder(resp.Body).Decode(&body)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || len(body.Items) != 1 {
			t.Fatalf("%s: status %d items %v err %v", path, resp.StatusCode, body.Items, err)
		}
	}
	resp, err := http.Get(srv.URL + "/v1/admin/users?limit=0")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("limit=0: status %d", resp.StatusCode)
	}
}
