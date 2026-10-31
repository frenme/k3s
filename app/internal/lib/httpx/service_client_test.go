package httpx

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type thing struct {
	Name string `json:"name"`
}

func newThingServer(t *testing.T) *ServiceClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			WriteJSON(w, http.StatusOK, thing{Name: "a"})
		case "/echo":
			body, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("method %s content-type %q", r.Method, r.Header.Get("Content-Type"))
			}
			_, _ = w.Write(body)
		case "/missing":
			WriteJSON(w, http.StatusNotFound, Error{Code: "thing_not_found", Message: "Thing not found", Details: []string{"id"}})
		case "/broken":
			WriteJSON(w, http.StatusInternalServerError, ErrInternal)
		case "/no-code":
			w.WriteHeader(http.StatusNotFound)
		case "/garbage":
			_, _ = w.Write([]byte("not json"))
		}
	}))
	t.Cleanup(srv.Close)
	return NewServiceClient("things", srv.URL, time.Second)
}

func TestServiceClientDecodesSuccess(t *testing.T) {
	c := newThingServer(t)
	got, err := Get[thing](t.Context(), c, "/ok")
	if err != nil || got.Name != "a" {
		t.Fatalf("get %+v %v", got, err)
	}
	echoed, err := Post[thing](t.Context(), c, "/echo", thing{Name: "b"})
	if err != nil || echoed.Name != "b" {
		t.Fatalf("post %+v %v", echoed, err)
	}
}

func TestServiceClientPassesClientErrorsThrough(t *testing.T) {
	_, err := Get[thing](t.Context(), newThingServer(t), "/missing")
	var apiErr Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound || apiErr.Message != "Thing not found" || len(apiErr.Details) != 1 {
		t.Fatalf("err %#v", err)
	}
	if !errors.Is(err, Error{Code: "thing_not_found"}) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("err %v does not match by code", err)
	}
}

func TestServiceClientReportsUnavailable(t *testing.T) {
	c := newThingServer(t)
	for _, path := range []string{"/broken", "/no-code", "/garbage"} {
		_, err := Get[thing](t.Context(), c, path)
		var apiErr Error
		if !errors.Is(err, ErrUnavailable) || !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadGateway || apiErr.Details[0] != "things" {
			t.Errorf("%s: %#v", path, err)
		}
	}

	dead := httptest.NewServer(http.NotFoundHandler())
	dead.Close()
	_, err := Get[thing](t.Context(), NewServiceClient("things", dead.URL, time.Second), "/ok")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("network error: %v", err)
	}
}
