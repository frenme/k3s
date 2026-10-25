package v1

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
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/orders/checkout"
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

func newServer(t *testing.T, u fakeUsers) string {
	t.Helper()
	mux := http.NewServeMux()
	svc := checkout.NewService(&fakeStore{byKey: map[string]model.Order{}}, u)
	NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(httpx.NewAPI(mux, "orders"))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func send(t *testing.T, url, body, key string) (int, map[string]any) {
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
	defer func() { _ = resp.Body.Close() }()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestCreateOrder(t *testing.T) {
	url := newServer(t, fakeUsers{}) + "/v1/orders"
	cases := map[string]int{
		`{"user_id":"` + knownUser + `","item":"book","amount_cents":1500}`:                 http.StatusCreated,
		`{"user_id":"22222222-2222-4222-8222-222222222222","item":"book","amount_cents":1}`: http.StatusUnprocessableEntity,
		`{"user_id":"not-a-uuid","item":"x","amount_cents":1}`:                              http.StatusUnprocessableEntity,
		`{"user_id":"` + knownUser + `","item":"","amount_cents":1}`:                        http.StatusUnprocessableEntity,
		`{"user_id":"` + knownUser + `","item":"x","amount_cents":-1}`:                      http.StatusUnprocessableEntity,
		`nope`: http.StatusBadRequest,
	}
	for body, want := range cases {
		code, out := send(t, url, body, "")
		if code != want {
			t.Fatalf("%s: status %d, want %d", body, code, want)
		}
		if msg, _ := out["error"].(string); code >= 400 && msg == "" {
			t.Fatalf("%s: no json error in %v", body, out)
		}
	}
}

func TestUsersServiceDown(t *testing.T) {
	url := newServer(t, fakeUsers{down: true}) + "/v1/orders"
	if code, _ := send(t, url, `{"user_id":"`+knownUser+`","item":"book","amount_cents":1}`, ""); code != http.StatusBadGateway {
		t.Fatalf("status %d", code)
	}
}

func TestIdempotency(t *testing.T) {
	url := newServer(t, fakeUsers{}) + "/v1/orders"
	body := `{"user_id":"` + knownUser + `","item":"book","amount_cents":1}`
	if code, _ := send(t, url, body, "k1"); code != http.StatusCreated {
		t.Fatalf("first status %d", code)
	}
	if code, _ := send(t, url, body, "k1"); code != http.StatusOK {
		t.Fatalf("second status %d", code)
	}
}

func TestGetOrder(t *testing.T) {
	base := newServer(t, fakeUsers{})
	_, created := send(t, base+"/v1/orders", `{"user_id":"`+knownUser+`","item":"book","amount_cents":5}`, "k-get")
	id, _ := created["id"].(string)
	for path, want := range map[string]int{"/v1/orders/" + id: http.StatusOK, "/v1/orders/nope": http.StatusNotFound} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
	}
}
