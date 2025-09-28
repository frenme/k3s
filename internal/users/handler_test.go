package users

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
)

type fakeStore struct {
	items map[string]User
}

func (f *fakeStore) List(ctx context.Context, limit int) ([]User, error) {
	out := []User{}
	for _, u := range f.items {
		out = append(out, u)
	}
	return out, nil
}

func (f *fakeStore) Get(ctx context.Context, id string) (User, error) {
	u, ok := f.items[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) Create(ctx context.Context, email, name string) (User, error) {
	for _, u := range f.items {
		if u.Email == email {
			return User{}, ErrDuplicate
		}
	}
	u := User{ID: "id-" + email, Email: email, Name: name, CreatedAt: time.Now()}
	f.items[u.ID] = u
	return u, nil
}

func newServer() *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(&fakeStore{items: map[string]User{}}, slog.New(slog.NewTextHandler(io.Discard, nil))).Mount(mux)
	return httptest.NewServer(mux)
}

func post(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCreateAndGet(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	resp := post(t, srv.URL+"/users", `{"email":"A@Example.com","name":"Ann"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if u.Email != "a@example.com" {
		t.Fatalf("email %q", u.Email)
	}

	got, err := http.Get(srv.URL + "/users/" + u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.StatusCode != http.StatusOK {
		t.Fatalf("status %d", got.StatusCode)
	}
}

func TestValidation(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	cases := []string{`{"email":"nope","name":"x"}`, `{"email":"a@b.co","name":""}`, `{bad`}
	for _, body := range cases {
		if resp := post(t, srv.URL+"/users", body); resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("%s: status %d", body, resp.StatusCode)
		}
	}
}

func TestDuplicateAndNotFound(t *testing.T) {
	srv := newServer()
	defer srv.Close()

	post(t, srv.URL+"/users", `{"email":"a@b.co","name":"x"}`)
	if resp := post(t, srv.URL+"/users", `{"email":"a@b.co","name":"y"}`); resp.StatusCode != http.StatusConflict {
		t.Fatalf("status %d", resp.StatusCode)
	}
	resp, _ := http.Get(srv.URL + "/users/missing")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
