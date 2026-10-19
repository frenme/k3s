package gateway

import (
	"net/http"
	"strings"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

type CORS struct {
	origins  map[string]bool
	wildcard bool
}

func NewCORS(origins []string) *CORS {
	c := &CORS{origins: map[string]bool{}}
	for _, o := range origins {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		switch o {
		case "":
		case "*":
			c.wildcard = true
		default:
			c.origins[o] = true
		}
	}
	return c
}

func (c *CORS) Middleware() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		if len(c.origins) == 0 && !c.wildcard {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			w.Header().Add("Vary", "Origin")
			if origin == "" || (!c.wildcard && !c.origins[origin]) {
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Expose-Headers", "X-Request-ID")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key, X-Request-ID")
				h.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
