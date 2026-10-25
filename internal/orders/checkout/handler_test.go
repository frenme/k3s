package checkout

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/google/uuid"
)

const knownUser = "11111111-1111-4111-8111-111111111111"

type fakeStore struct {
	byKey map[string]model.Order
}

func (f *fakeStore) List(ctx context.Context, userID string, limit int) ([]model.Order, error) {
	return nil, nil
}

func (f *fakeStore) Get(ctx context.Context, id string) (model.Order, error) {
	for _, o := range f.byKey {
		if o.ID == id {
			return o, nil
		}
	}
	return model.Order{}, model.ErrNotFound
}

func (f *fakeStore) Create(ctx context.Context, o model.Order, key string) (model.Order, bool, error) {
	if existing, ok := f.byKey[key]; ok && key != "" {
		return existing, false, nil
	}
	o.ID = uuid.NewString()
	o.CreatedAt = time.Now()
	if key != "" {
		f.byKey[key] = o
	}
	return o, true, nil
}

type fakeUsers struct {
	down bool
}

func (f fakeUsers) Get(ctx context.Context, id string) (users.User, error) {
	switch {
	case f.down:
		return users.User{}, errors.New("connection refused")
	case id == knownUser:
		return users.User{ID: id}, nil
	default:
		return users.User{}, users.ErrNotFound
	}
}

func newServer(u fakeUsers) *httptest.Server {
	mux := http.NewServeMux()
	svc := NewService(&fakeStore{byKey: map[string]model.Order{}}, u)
	NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Mount(mux)
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
	srv := newServer(fakeUsers{})
	defer srv.Close()

	cases := map[string]int{
		`{"user_id":"` + knownUser + `","item":"book","amount_cents":1500}`:                 http.StatusCreated,
		`{"user_id":"22222222-2222-4222-8222-222222222222","item":"book","amount_cents":1}`: http.StatusUnprocessableEntity,
		`{"user_id":"not-a-uuid","item":"x","amount_cents":1}`:                              http.StatusBadRequest,
		`{"user_id":"` + knownUser + `","item":"","amount_cents":1}`:                        http.StatusBadRequest,
		`{"user_id":"` + knownUser + `","item":"x","amount_cents":-1}`:                      http.StatusBadRequest,
		`nope`: http.StatusBadRequest,
	}
	for body, want := range cases {
		if resp := send(t, srv.URL+"/orders", body, ""); resp.StatusCode != want {
			t.Fatalf("%s: status %d, want %d", body, resp.StatusCode, want)
		}
	}
}

func TestUsersServiceDown(t *testing.T) {
	srv := newServer(fakeUsers{down: true})
	defer srv.Close()

	resp := send(t, srv.URL+"/orders", `{"user_id":"`+knownUser+`","item":"book","amount_cents":1}`, "")
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestIdempotency(t *testing.T) {
	srv := newServer(fakeUsers{})
	defer srv.Close()

	body := `{"user_id":"` + knownUser + `","item":"book","amount_cents":1}`
	if resp := send(t, srv.URL+"/orders", body, "k1"); resp.StatusCode != http.StatusCreated {
		t.Fatalf("first status %d", resp.StatusCode)
	}
	if resp := send(t, srv.URL+"/orders", body, "k1"); resp.StatusCode != http.StatusOK {
		t.Fatalf("second status %d", resp.StatusCode)
	}
}

func TestGetOrder(t *testing.T) {
	srv := newServer(fakeUsers{})
	defer srv.Close()

	resp := send(t, srv.URL+"/orders", `{"user_id":"`+knownUser+`","item":"book","amount_cents":5}`, "k-get")
	var created model.Order
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]int{"/orders/" + created.ID: http.StatusOK, "/orders/nope": http.StatusNotFound} {
		got, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		if got.StatusCode != want {
			t.Fatalf("%s: status %d", path, got.StatusCode)
		}
	}
}
