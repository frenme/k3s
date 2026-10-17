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
	hops   int
}

func NewLimiter(client *redis.Client, limit int, window time.Duration, trustedHops int) *Limiter {
	if window < time.Second {
		window = time.Second
	}
	return &Limiter{client: client, limit: limit, window: window, hops: trustedHops}
}

func (l *Limiter) Middleware() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		if l == nil || l.client == nil || l.limit <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bucket := time.Now().Unix() / int64(l.window.Seconds())
			key := "rl:" + clientIP(r, l.hops) + ":" + strconv.FormatInt(bucket, 10)
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

func clientIP(r *http.Request, trustedHops int) string {
	if trustedHops > 0 {
		var chain []string
		for _, v := range r.Header.Values("X-Forwarded-For") {
			for _, part := range strings.Split(v, ",") {
				if part = strings.TrimSpace(part); part != "" {
					chain = append(chain, part)
				}
			}
		}
		if len(chain) >= trustedHops {
			return chain[len(chain)-trustedHops]
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
