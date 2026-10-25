package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"id": r.PathValue("id")})
	})
	mux.HandleFunc("GET /dir/", func(w http.ResponseWriter, r *http.Request) {})
	h := JSONErrors(mux)

	cases := []struct {
		method, path string
		status       int
		body, allow  string
	}{
		{http.MethodGet, "/items/1", http.StatusOK, "", ""},
		{http.MethodGet, "/nope", http.StatusNotFound, "not_found", ""},
		{http.MethodDelete, "/items/1", http.StatusMethodNotAllowed, "method_not_allowed", "GET, HEAD"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
		if rec.Code != c.status {
			t.Fatalf("%s %s: status %d", c.method, c.path, rec.Code)
		}
		if got := rec.Header().Get("Allow"); got != c.allow {
			t.Fatalf("%s %s: allow %q", c.method, c.path, got)
		}
		if c.body == "" {
			continue
		}
		var body Error
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != c.body {
			t.Fatalf("%s %s: body %q", c.method, c.path, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s %s: content type %q", c.method, c.path, ct)
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dir", nil))
	if rec.Code/100 != 3 || rec.Header().Get("Location") != "/dir/" {
		t.Fatalf("redirect: status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
}
