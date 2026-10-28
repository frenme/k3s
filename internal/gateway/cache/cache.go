package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	maxBody     = 1 << 20
	staleFactor = 20
)

var lookups = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "gateway_cache_requests_total",
	Help: "Gateway response cache lookups by result.",
}, []string{"result"})

type entry struct {
	status int
	header http.Header
	body   []byte
	etag   string
	stored time.Time
}

type flight struct {
	done chan struct{}
}

type Cache struct {
	max int
	now func() time.Time

	mu      sync.Mutex
	entries map[string]*entry
	flights map[string]*flight
}

func New(maxEntries int) *Cache {
	return &Cache{max: maxEntries, now: time.Now, entries: map[string]*entry{}, flights: map[string]*flight{}}
}

func (c *Cache) Middleware() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rule := policy.From(r.Context()).Rule
			if r.Method != http.MethodGet || rule.CacheTTL <= 0 || rule.Access != httpx.AccessPublic {
				next.ServeHTTP(w, r)
				return
			}
			c.serve(w, r, next, rule.CacheTTL)
		})
	}
}

func (c *Cache) serve(w http.ResponseWriter, r *http.Request, next http.Handler, ttl time.Duration) {
	key := r.URL.RequestURI()
	for {
		c.mu.Lock()
		e := c.entries[key]
		if e != nil && c.now().Sub(e.stored) < ttl {
			c.mu.Unlock()
			lookups.WithLabelValues("hit").Inc()
			write(w, r, e, "HIT", ttl)
			return
		}
		if f, busy := c.flights[key]; busy {
			c.mu.Unlock()
			select {
			case <-f.done:
				continue
			case <-r.Context().Done():
				return
			}
		}
		f := &flight{done: make(chan struct{})}
		c.flights[key] = f
		c.mu.Unlock()

		c.fill(w, r, next, key, ttl, e)
		c.mu.Lock()
		delete(c.flights, key)
		c.mu.Unlock()
		close(f.done)
		return
	}
}

func (c *Cache) fill(w http.ResponseWriter, r *http.Request, next http.Handler, key string, ttl time.Duration, stale *entry) {
	rec := &recorder{header: http.Header{}, status: http.StatusOK}
	next.ServeHTTP(rec, r)

	if rec.status >= http.StatusInternalServerError && stale != nil && c.now().Sub(stale.stored) < ttl*staleFactor {
		lookups.WithLabelValues("stale").Inc()
		write(w, r, stale, "STALE", ttl)
		return
	}
	if rec.status != http.StatusOK || rec.overflow {
		lookups.WithLabelValues("bypass").Inc()
		rec.flush(w)
		return
	}
	sum := sha256.Sum256(rec.body.Bytes())
	e := &entry{
		status: rec.status,
		header: pick(rec.header),
		body:   rec.body.Bytes(),
		etag:   `"` + hex.EncodeToString(sum[:8]) + `"`,
		stored: c.now(),
	}
	c.store(key, e)
	lookups.WithLabelValues("miss").Inc()
	write(w, r, e, "MISS", ttl)
}

func (c *Cache) store(key string, e *entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.max {
		var oldestKey string
		var oldest time.Time
		for k, v := range c.entries {
			if oldestKey == "" || v.stored.Before(oldest) {
				oldestKey, oldest = k, v.stored
			}
		}
		delete(c.entries, oldestKey)
	}
	c.entries[key] = e
}

func write(w http.ResponseWriter, r *http.Request, e *entry, result string, ttl time.Duration) {
	h := w.Header()
	for k, v := range e.header {
		h[k] = v
	}
	h.Set("ETag", e.etag)
	h.Set("X-Cache", result)
	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "public, max-age="+strconv.Itoa(int(ttl.Seconds())))
	}
	if r.Header.Get("If-None-Match") == e.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	h.Set("Content-Length", strconv.Itoa(len(e.body)))
	w.WriteHeader(e.status)
	_, _ = w.Write(e.body)
}

func pick(h http.Header) http.Header {
	out := http.Header{}
	for _, k := range []string{"Content-Type", "Cache-Control", "Content-Language"} {
		if v := h.Values(k); len(v) > 0 {
			out[k] = v
		}
	}
	return out
}

type recorder struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) { r.status = status }

func (r *recorder) Write(b []byte) (int, error) {
	if r.body.Len()+len(b) > maxBody {
		r.overflow = true
		return len(b), nil
	}
	return r.body.Write(b)
}

func (r *recorder) flush(w http.ResponseWriter) {
	for k, v := range r.header {
		w.Header()[k] = v
	}
	w.WriteHeader(r.status)
	_, _ = w.Write(r.body.Bytes())
}
