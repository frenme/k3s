package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

func TestAdminMiddleware(t *testing.T) {
	var gotAuth string
	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
	}), AdminMiddleware())

	cases := []struct {
		path, user, password string
		want                 int
	}{
		{"/api/v1/users", "", "", http.StatusOK},
		{"/api/v1/admin/users", "", "", http.StatusUnauthorized},
		{"/api/v1/admin/users", adminUser, "wrong", http.StatusUnauthorized},
		{"/api/v2/admin/orders", "root", adminPassword, http.StatusUnauthorized},
		{"/api/v1/admin/users", adminUser, adminPassword, http.StatusOK},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		if c.user != "" {
			req.SetBasicAuth(c.user, c.password)
		}
		rec := httptest.NewRecorder()
		gotAuth = "unset"
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Fatalf("%s %s: status %d", c.path, c.user, rec.Code)
		}
		if c.want == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
			t.Fatalf("%s: no WWW-Authenticate", c.path)
		}
		if c.want == http.StatusOK && c.user != "" && gotAuth != "" {
			t.Fatalf("%s: credentials leaked upstream", c.path)
		}
	}
}
