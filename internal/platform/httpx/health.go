package httpx

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/egor/k3s-base/internal/platform/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Check func(ctx context.Context) error

type Health struct {
	ready  atomic.Bool
	checks map[string]Check
}

func NewHealth() *Health {
	h := &Health{checks: map[string]Check{}}
	h.ready.Store(true)
	return h
}

func (h *Health) Add(name string, c Check) {
	h.checks[name] = c
}

func (h *Health) SetReady(v bool) {
	h.ready.Store(v)
}

func (h *Health) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !h.ready.Load() {
			WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "draining"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		failed := map[string]string{}
		for name, c := range h.checks {
			if err := c(ctx); err != nil {
				failed[name] = err.Error()
			}
		}
		if len(failed) > 0 {
			WriteJSON(w, http.StatusServiceUnavailable, failed)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"version": version.Version, "commit": version.Commit})
	})
	mux.Handle("GET /metrics", promhttp.Handler())
}
