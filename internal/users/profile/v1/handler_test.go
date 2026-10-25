package v1

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

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/users/model"
	"github.com/egor/k3s-base/internal/users/profile"
	"github.com/google/uuid"
)

type fakeStore struct {
	items map[string]model.User
}

func (f *fakeStore) List(ctx context.Context, limit int) ([]model.User, error) {
	out := []model.User{}
	for _, u := range f.items {
		out = append(out, u)
	}
	return out, nil
}

func (f *fakeStore) Get(ctx context.Context, id string) (model.User, error) {
	u, ok := f.items[id]
	if !ok {
		return model.User{}, model.ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) Create(ctx context.Context, email, name string) (model.User, error) {
	for _, u := range f.items {
		if u.Email == email {
			return model.User{}, model.ErrDuplicate
		}
	}
	u := model.User{ID: uuid.NewString(), Email: email, Name: name, CreatedAt: time.Now()}
	f.items[u.ID] = u
	return u, nil
}

func newServers(t *testing.T) (public, internal string) {
	t.Helper()
	pubMux, inMux := http.NewServeMux(), http.NewServeMux()
	svc := profile.NewService(&fakeStore{items: map[string]model.User{}})
	NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(httpx.NewAPI(pubMux, "users"), httpx.NewInternalAPI(inMux, "users"))
	pub, in := httptest.NewServer(pubMux), httptest.NewServer(inMux)
	t.Cleanup(pub.Close)
	t.Cleanup(in.Close)
	return pub.URL, in.URL
}

func post(t *testing.T, url, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func status(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestCreateAndGet(t *testing.T) {
	public, internal := newServers(t)

	code, body := post(t, public+"/v1/users", `{"email":"A@Example.com","name":"Ann"}`)
	if code != http.StatusCreated || body["email"] != "a@example.com" {
		t.Fatalf("create %d %v", code, body)
	}
	id, _ := body["id"].(string)
	if got := status(t, public+"/v1/users/"+id); got != http.StatusOK {
		t.Fatalf("public get %d", got)
	}
	if got := status(t, internal+"/internal/v1/users/"+id); got != http.StatusOK {
		t.Fatalf("internal get %d", got)
	}
}

func TestInternalRoutesStayOffThePublicAPI(t *testing.T) {
	public, internal := newServers(t)
	if got := status(t, public+"/internal/v1/users/"+uuid.NewString()); got != http.StatusNotFound {
		t.Fatalf("public api serves internal route: %d", got)
	}
	if got := status(t, internal+"/v1/users"); got != http.StatusNotFound {
		t.Fatalf("internal api serves public route: %d", got)
	}
}

func TestErrorsAreJSON(t *testing.T) {
	public, _ := newServers(t)
	post(t, public+"/v1/users", `{"email":"a@b.co","name":"x"}`)

	cases := map[string]int{
		`{"email":"nope","name":"x"}`:   http.StatusUnprocessableEntity,
		`{"email":"c@b.co","name":""}`:  http.StatusUnprocessableEntity,
		`{"email":"a@b.co","name":"y"}`: http.StatusConflict,
		`{bad`:                          http.StatusBadRequest,
	}
	for in, want := range cases {
		code, body := post(t, public+"/v1/users", in)
		if msg, _ := body["error"].(string); code != want || msg == "" {
			t.Fatalf("%s: status %d body %v", in, code, body)
		}
	}
	for _, id := range []string{"missing", uuid.NewString()} {
		if got := status(t, public+"/v1/users/"+id); got != http.StatusNotFound {
			t.Fatalf("%s: status %d", id, got)
		}
	}
	if got := status(t, public+"/v1/users?limit=0"); got != http.StatusUnprocessableEntity {
		t.Fatalf("limit=0: status %d", got)
	}
}

func TestDocs(t *testing.T) {
	public, _ := newServers(t)
	for _, path := range []string{"/v1/users/openapi.json", "/v1/users/docs"} {
		if got := status(t, public+path); got != http.StatusOK {
			t.Fatalf("%s: status %d", path, got)
		}
	}
}
