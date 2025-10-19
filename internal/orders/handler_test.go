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

