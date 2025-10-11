package orders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUsersClientExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/known":
			w.WriteHeader(http.StatusOK)
		case "/users/broken":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := NewUsersClient(srv.URL)
	ok, err := c.Exists(context.Background(), "known")
	if err != nil || !ok {
		t.Fatalf("known: %v %v", ok, err)
	}
	ok, err = c.Exists(context.Background(), "missing")
	if err != nil || ok {
		t.Fatalf("missing: %v %v", ok, err)
	}
	if _, err = c.Exists(context.Background(), "broken"); err == nil {
		t.Fatal("expected error")
	}
}
