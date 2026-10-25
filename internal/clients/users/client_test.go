package users

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/users/known":
			_, _ = w.Write([]byte(`{"id":"known","email":"a@b.co","name":"Ann"}`))
		case "/internal/v1/users/broken":
			w.WriteHeader(http.StatusInternalServerError)
		case "/internal/v1/users/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"user_not_found","message":"User not found"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"not_found","message":"Not found"}`))
		}
	}))
	defer srv.Close()

	c := New(srv.URL)
	u, err := c.Get(t.Context(), "known")
	if err != nil || u.ID != "known" || u.Email != "a@b.co" {
		t.Fatalf("known: %+v %v", u, err)
	}
	if _, err := c.Get(t.Context(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	for _, id := range []string{"broken", "wrong-path"} {
		if _, err := c.Get(t.Context(), id); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("%s: %v", id, err)
		}
	}
}
