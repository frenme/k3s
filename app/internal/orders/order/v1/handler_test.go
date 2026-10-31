package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/idempotency"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/egor/k3s-base/internal/orders/order"
	"github.com/google/uuid"
)

const (
	knownUser   = "11111111-1111-4111-8111-111111111111"
	unknownUser = "22222222-2222-4222-8222-222222222222"
)

type fakeStore struct {
	byKey  map[string]model.Order
	hashes map[string]string
}

func (f *fakeStore) List(ctx context.Context, userID string, limit int, after *pagination.Cursor) ([]model.Order, error) {
	out := []model.Order{}
	for _, o := range f.byKey {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeStore) Get(ctx context.Context, id string) (model.Order, error) {
	for _, o := range f.byKey {
		if o.ID == id {
			return o, nil
		}
	}
	return model.Order{}, model.ErrOrderNotFound
}

func (f *fakeStore) Create(ctx context.Context, o model.Order, idem idempotency.Request) (model.Order, bool, error) {
	if existing, ok := f.byKey[idem.Key]; ok && !idem.Empty() {
		if err := idem.Verify(f.hashes[idem.Key]); err != nil {
			return model.Order{}, false, err
		}
		return existing, false, nil
	}
	o.ID = uuid.NewString()
	o.CreatedAt = time.Now()
	if !idem.Empty() {
		f.byKey[idem.Key] = o
		f.hashes[idem.Key] = idem.Fingerprint
	}
	return o, true, nil
}

type fakeUsers struct {
	down bool
}

func (f fakeUsers) Get(ctx context.Context, id string) (users.User, error) {
	switch {
	case f.down:
		return users.User{}, httpx.ErrUnavailable
	case id == knownUser:
		return users.User{ID: id}, nil
	default:
		return users.User{}, users.ErrNotFound
	}
}

func newServer(t *testing.T, u fakeUsers) string {
	t.Helper()
	mux := http.NewServeMux()
	svc := order.NewService(&fakeStore{byKey: map[string]model.Order{}, hashes: map[string]string{}}, u)
	NewHandler(svc).Register(httpx.NewAPI(mux, "orders"))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL
}

func do(t *testing.T, method, url, user, body, key string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.Header.Set(httpx.UserIDHeader, user)
	}
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

func send(t *testing.T, url, user, body, key string) (int, map[string]any) {
	t.Helper()
	return do(t, http.MethodPost, url, user, body, key)
}

func TestCreateOrder(t *testing.T) {
	url := newServer(t, fakeUsers{}) + "/v1/orders"
	cases := []struct {
		name, user, body string
		status           int
		code             string
	}{
		{"ok", knownUser, `{"item":"book","amount_cents":1500}`, http.StatusOK, ""},
		{"unknown user", unknownUser, `{"item":"book","amount_cents":1}`, http.StatusBadRequest, "unknown_user"},
		{"subject is not a uuid", "not-a-uuid", `{"item":"book","amount_cents":1}`, http.StatusBadRequest, "unknown_user"},
		{"no user header", "", `{"item":"book","amount_cents":1}`, http.StatusUnauthorized, "unauthorized"},
		{"owner is not taken from the body", knownUser, `{"user_id":"` + unknownUser + `","item":"book","amount_cents":1}`, http.StatusBadRequest, "invalid_input"},
		{"empty item", knownUser, `{"item":"","amount_cents":1}`, http.StatusBadRequest, "invalid_input"},
		{"blank item", knownUser, `{"item":"   ","amount_cents":1}`, http.StatusBadRequest, "invalid_input"},
		{"negative amount", knownUser, `{"item":"x","amount_cents":-1}`, http.StatusBadRequest, "invalid_input"},
		{"not json", knownUser, `nope`, http.StatusBadRequest, "bad_request"},
	}
	for _, c := range cases {
		status, out := send(t, url, c.user, c.body, "")
		if code, _ := out["code"].(string); status != c.status || code != c.code {
			t.Fatalf("%s: status %d body %v, want %d %q", c.name, status, out, c.status, c.code)
		}
	}
}

func TestUsersServiceDown(t *testing.T) {
	url := newServer(t, fakeUsers{down: true}) + "/v1/orders"
	if status, out := send(t, url, knownUser, `{"item":"book","amount_cents":1}`, ""); status != http.StatusBadGateway || out["code"] != "upstream_unavailable" {
		t.Fatalf("status %d body %v", status, out)
	}
}

func TestIdempotency(t *testing.T) {
	url := newServer(t, fakeUsers{}) + "/v1/orders"
	body := `{"item":"book","amount_cents":1}`
	code, first := send(t, url, knownUser, body, "k1")
	if code != http.StatusOK {
		t.Fatalf("first status %d", code)
	}
	code, replay := send(t, url, knownUser, body, "k1")
	if code != http.StatusOK || replay["id"] != first["id"] {
		t.Fatalf("a replay must return the first order: status %d, %v and %v", code, first, replay)
	}
}

func TestIdempotencyKeyReuseWithDifferentBody(t *testing.T) {
	url := newServer(t, fakeUsers{}) + "/v1/orders"
	if code, _ := send(t, url, knownUser, `{"item":"book","amount_cents":1}`, "k2"); code != http.StatusOK {
		t.Fatalf("first status %d", code)
	}
	if code, out := send(t, url, knownUser, `{"item":"book","amount_cents":2}`, "k2"); code != http.StatusBadRequest || out["code"] != "idempotency_key_reused" {
		t.Fatalf("status %d body %v", code, out)
	}
}

func TestOrdersBelongToTheirOwner(t *testing.T) {
	base := newServer(t, fakeUsers{})
	_, created := send(t, base+"/v1/orders", knownUser, `{"item":"book","amount_cents":5}`, "k-get")
	id, _ := created["id"].(string)
	if id == "" || created["user_id"] != knownUser {
		t.Fatalf("created %v", created)
	}

	cases := []struct {
		name, path, user string
		status           int
	}{
		{"owner reads it", "/v1/orders/" + id, knownUser, http.StatusOK},
		{"someone else gets a 404", "/v1/orders/" + id, unknownUser, http.StatusNotFound},
		{"no user header", "/v1/orders/" + id, "", http.StatusUnauthorized},
		{"not a uuid", "/v1/orders/nope", knownUser, http.StatusNotFound},
	}
	for _, c := range cases {
		if status, out := do(t, http.MethodGet, base+c.path, c.user, "", ""); status != c.status {
			t.Fatalf("%s: status %d body %v", c.name, status, out)
		}
	}

	itemsOf := func(user string) int {
		status, out := do(t, http.MethodGet, base+"/v1/orders", user, "", "")
		items, _ := out["items"].([]any)
		if status != http.StatusOK {
			t.Fatalf("list as %q: status %d body %v", user, status, out)
		}
		return len(items)
	}
	if got := itemsOf(knownUser); got != 1 {
		t.Fatalf("owner sees %d orders", got)
	}
	if got := itemsOf(unknownUser); got != 0 {
		t.Fatalf("someone else sees %d orders", got)
	}
	if got := itemsOf("not-a-uuid"); got != 0 {
		t.Fatalf("a subject that is not a uuid sees %d orders", got)
	}
	if status, _ := do(t, http.MethodGet, base+"/v1/orders", "", "", ""); status != http.StatusUnauthorized {
		t.Fatalf("no user header: status %d", status)
	}
}
