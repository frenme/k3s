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
		case "/internal/users/known":
			_, _ = w.Write([]byte(`{"id":"known","email":"a@b.co","name":"Ann"}`))
		case "/internal/users/broken":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
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
	if _, err := c.Get(t.Context(), "broken"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("broken: %v", err)
	}
}
