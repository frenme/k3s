package profile

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

	"github.com/egor/k3s-base/internal/users/model"
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

func newServers() (*httptest.Server, *httptest.Server) {
	public, internal := http.NewServeMux(), http.NewServeMux()
	svc := NewService(&fakeStore{items: map[string]model.User{}})
	NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Mount(public, internal)
	return httptest.NewServer(public), httptest.NewServer(internal)
}

func post(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
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
	public, internal := newServers()
	defer public.Close()
	defer internal.Close()

	resp := post(t, public.URL+"/users", `{"email":"A@Example.com","name":"Ann"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var u model.User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if u.Email != "a@example.com" {
		t.Fatalf("email %q", u.Email)
	}
	if got := status(t, public.URL+"/users/"+u.ID); got != http.StatusOK {
		t.Fatalf("public get %d", got)
	}
	if got := status(t, internal.URL+"/internal/users/"+u.ID); got != http.StatusOK {
		t.Fatalf("internal get %d", got)
	}
}

func TestInternalRoutesStayOffThePublicMux(t *testing.T) {
	public, internal := newServers()
	defer public.Close()
	defer internal.Close()

	if got := status(t, public.URL+"/internal/users/"+uuid.NewString()); got != http.StatusNotFound {
		t.Fatalf("public mux serves internal route: %d", got)
	}
	if got := status(t, internal.URL+"/users"); got != http.StatusNotFound {
		t.Fatalf("internal mux serves public route: %d", got)
	}
}

func TestValidation(t *testing.T) {
	public, internal := newServers()
	defer public.Close()
	defer internal.Close()

	for _, body := range []string{`{"email":"nope","name":"x"}`, `{"email":"a@b.co","name":""}`, `{bad`} {
		if resp := post(t, public.URL+"/users", body); resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status %d", body, resp.StatusCode)
		}
	}
}

func TestDuplicateAndNotFound(t *testing.T) {
	public, internal := newServers()
	defer public.Close()
	defer internal.Close()

	post(t, public.URL+"/users", `{"email":"a@b.co","name":"x"}`)
	if resp := post(t, public.URL+"/users", `{"email":"a@b.co","name":"y"}`); resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d", resp.StatusCode)
	}
	for _, id := range []string{"missing", "33333333-3333-4333-8333-333333333333"} {
		if got := status(t, public.URL+"/users/"+id); got != http.StatusNotFound {
			t.Fatalf("%s: status %d", id, got)
		}
	}
}
