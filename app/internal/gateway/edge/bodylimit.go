package edge

import (
	"net/http"

	"github.com/egor/k3s-base/internal/gateway/model"
	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

func BodyLimit(defaultMax int64) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit := policy.From(r.Context()).Rule.MaxBody
			if limit <= 0 {
				limit = defaultMax
			}
			if limit > 0 {
				if r.ContentLength > limit {
					httpx.WriteError(w, model.ErrPayloadTooLarge)
					return
				}
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
