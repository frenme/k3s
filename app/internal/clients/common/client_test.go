package common

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

func TestBool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v1/common/app-config/on":
			_, _ = w.Write([]byte(`{"key":"on","value":true}`))
		case "/internal/v1/common/app-config/missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"app_config_not_found","message":"AppConfig not found"}`))
		default:
			_, _ = w.Write([]byte(`{"key":"text","value":"yes"}`))
		}
	}))
	defer srv.Close()
	c := New(srv.URL)

	if v, err := c.Bool(t.Context(), "on"); err != nil || !v {
		t.Fatalf("on: %v %v", v, err)
	}
	_, err := c.Bool(t.Context(), "missing")
	if err == nil {
		t.Fatal("a missing key must fail")
	}
	if _, isHTTPError := errors.AsType[httpx.Error](err); isHTTPError {
		t.Fatalf("a missing key is a setup problem of this app and must not reach the caller as %v", err)
	}
	if _, err := c.Bool(t.Context(), "text"); err == nil {
		t.Fatal("a string value must not be read as bool")
	}
}
