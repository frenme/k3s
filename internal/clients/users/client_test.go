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
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"user_not_found","message":"User not found"}`))
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
}

func TestRoles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/users/u1/roles":
			_, _ = w.Write([]byte(`{"roles":["admin","user"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"user_not_found","message":"x"}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL)
	if roles, err := c.Roles(t.Context(), "u1"); err != nil || len(roles) != 2 || roles[0] != "admin" {
		t.Fatalf("roles %v err %v", roles, err)
	}
	if _, err := c.Roles(t.Context(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err %v", err)
	}
}
