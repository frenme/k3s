package common

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/common/config/on":
			_, _ = w.Write([]byte(`{"key":"on","value":true}`))
		case "/internal/v1/common/config/text":
			_, _ = w.Write([]byte(`{"key":"text","value":"yes"}`))
		case "/internal/v1/common/config/broken":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"internal_error","message":"x"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"config_not_found","message":"x"}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL)

	if v, err := c.Bool(t.Context(), "on"); err != nil || !v {
		t.Fatalf("on: %v %v", v, err)
	}
	if _, err := c.Bool(t.Context(), "missing"); !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("missing: %v", err)
	}
	for _, key := range []string{"text", "broken"} {
		if _, err := c.Bool(t.Context(), key); err == nil || errors.Is(err, ErrConfigNotFound) {
			t.Fatalf("%s: %v", key, err)
		}
	}
}
