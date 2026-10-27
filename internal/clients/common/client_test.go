package common

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/common/app-config/on":
			_, _ = w.Write([]byte(`{"key":"on","value":true}`))
		default:
			_, _ = w.Write([]byte(`{"key":"text","value":"yes"}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL)

	if v, err := c.Bool(t.Context(), "on"); err != nil || !v {
		t.Fatalf("on: %v %v", v, err)
	}
	if _, err := c.Bool(t.Context(), "text"); err == nil {
		t.Fatal("a string value must not be read as bool")
	}
}
