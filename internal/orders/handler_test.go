package orders

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"time"

	"github.com/google/uuid"
)

type fakeStore struct {
	byKey map[string]Order
	count int
}

func (f *fakeStore) List(ctx context.Context, userID string, limit int) ([]Order, error) {
	return nil, nil
}

func (f *fakeStore) Get(ctx context.Context, id string) (Order, error) {
	for _, o := range f.byKey {
		if o.ID == id {
			return o, nil
		}
	}
	return Order{}, ErrNotFound
}

func (f *fakeStore) Create(ctx context.Context, o Order, key string) (Order, bool, error) {
	if key != "" {
		if existing, ok := f.byKey[key]; ok {
			return existing, false, nil
		}
	}
	f.count++
	o.ID = uuid.NewString()
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
	users := fakeUsers{known: map[string]bool{"11111111-1111-4111-8111-111111111111": true}}
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

	resp := send(t, srv.URL+"/orders", `{"user_id":"11111111-1111-4111-8111-111111111111","item":"book","amount_cents":1500}`, "")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestUnknownUser(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	resp := send(t, srv.URL+"/orders", `{"user_id":"22222222-2222-4222-8222-222222222222","item":"book","amount_cents":1}`, "")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestIdempotency(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	body := `{"user_id":"11111111-1111-4111-8111-111111111111","item":"book","amount_cents":1}`
	if resp := send(t, srv.URL+"/orders", body, "k1"); resp.StatusCode != http.StatusCreated {
		t.Fatalf("first status %d", resp.StatusCode)
	}
	if resp := send(t, srv.URL+"/orders", body, "k1"); resp.StatusCode != http.StatusOK {
		t.Fatalf("second status %d", resp.StatusCode)
	}
}

func TestValidation(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	for _, body := range []string{`{"user_id":"not-a-uuid","item":"x","amount_cents":1}`, `{"user_id":"11111111-1111-4111-8111-111111111111","item":"","amount_cents":1}`, `{"user_id":"11111111-1111-4111-8111-111111111111","item":"x","amount_cents":-1}`, `nope`} {
		if resp := send(t, srv.URL+"/orders", body, ""); resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status %d", body, resp.StatusCode)
		}
	}
}

func TestGetOrder(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	resp := send(t, srv.URL+"/orders", `{"user_id":"11111111-1111-4111-8111-111111111111","item":"book","amount_cents":5}`, "k-get")
	var created Order
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	got, err := http.Get(srv.URL + "/orders/" + created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.StatusCode != http.StatusOK {
		t.Fatalf("status %d", got.StatusCode)
	}
	missing, _ := http.Get(srv.URL + "/orders/nope")
	if missing.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", missing.StatusCode)
	}
}
