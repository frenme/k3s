package orders

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	byKey map[string]Order
	count int
}

func (f *fakeStore) List(ctx context.Context, userID string, limit int) ([]Order, error) {
	return nil, nil
}

func (f *fakeStore) Create(ctx context.Context, o Order, key string) (Order, bool, error) {
	if key != "" {
		if existing, ok := f.byKey[key]; ok {
			return existing, false, nil
		}
	}
	f.count++
	o.ID = "order-" + strings.Repeat("x", f.count)
	o.CreatedAt = time.Now()
	if key != "" {
		f.byKey[key] = o
	}
	return o, true, nil
}

type fakeUsers struct {
	known map[string]bool
}

func (f fakeUsers) Exists(ctx context.Context, id string) (bool, error) {
	return f.known[id], nil
}

func newServer() *httptest.Server {
	mux := http.NewServeMux()
	store := &fakeStore{byKey: map[string]Order{}}
	users := fakeUsers{known: map[string]bool{"u1": true}}
	NewHandler(store, users, slog.New(slog.NewTextHandler(io.Discard, nil))).Mount(mux)
	return httptest.NewServer(mux)
}

func send(t *testing.T, url, body, key string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCreateOrder(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	resp := send(t, srv.URL+"/orders", `{"user_id":"u1","item":"book","amount_cents":1500}`, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestUnknownUser(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	resp := send(t, srv.URL+"/orders", `{"user_id":"nope","item":"book","amount_cents":1}`, "")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

