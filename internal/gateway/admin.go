package gateway

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

const (
	adminUser     = "admin"
	adminPassword = "admin-dev-password"
)

var errAdminUnauthorized = httpx.Error{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Admin credentials required"}

func AdminMiddleware() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isAdminPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			user, password, ok := r.BasicAuth()
			if !ok || subtle.ConstantTimeCompare([]byte(user+":"+password), []byte(adminUser+":"+adminPassword)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
				httpx.WriteError(w, errAdminUnauthorized)
				return
			}
			r.Header.Del("Authorization")
			next.ServeHTTP(w, r)
		})
	}
}

func isAdminPath(path string) bool {
	_, rest, _ := strings.Cut(strings.TrimPrefix(path, "/api/"), "/")
	name, _, _ := strings.Cut(rest, "/")
	return name == "admin"
}
