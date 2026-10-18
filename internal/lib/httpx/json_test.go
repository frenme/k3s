package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, http.StatusCreated, map[string]string{"a": "b"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}
	if strings.TrimSpace(rec.Body.String()) != `{"a":"b"}` {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusTeapot, "nope")
	if !strings.Contains(rec.Body.String(), `"error":"nope"`) {
		t.Fatalf("body %q", rec.Body.String())
	}
}

func TestReadJSONRejectsUnknownFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1,"extra":2}`))
	var dst struct {
		A int `json:"a"`
	}
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 1<<20)
	if err := ReadJSON(req, &dst); err == nil {
		t.Fatal("expected error")
	}
}
