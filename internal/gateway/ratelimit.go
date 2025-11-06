package gateway

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/egor/k3s-base/internal/platform/httpx"
	"github.com/redis/go-redis/v9"
)

type Limiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

func NewLimiter(client *redis.Client, limit int, window time.Duration) *Limiter {
	return &Limiter{client: client, limit: limit, window: window}
}

func (l *Limiter) Middleware() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		if l == nil || l.client == nil || l.limit <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bucket := time.Now().Unix() / int64(l.window.Seconds())
			key := "rl:" + clientIP(r) + ":" + strconv.FormatInt(bucket, 10)
			pipe := l.client.Pipeline()
			incr := pipe.Incr(r.Context(), key)
			pipe.Expire(r.Context(), key, l.window*2)
			if _, err := pipe.Exec(r.Context()); err != nil {
				next.ServeHTTP(w, r)
				return
			}
			if int(incr.Val()) > l.limit {
				w.Header().Set("Retry-After", strconv.Itoa(int(l.window.Seconds())))
				httpx.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

