package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

type Middleware func(http.Handler) http.Handler

const RequestIDHeader = "X-Request-ID"

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests.",
	}, []string{"method", "route", "code"})
	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})
)

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func Trace(operation string, public bool) Middleware {
	return func(next http.Handler) http.Handler {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
				w.Header().Set(RequestIDHeader, sc.TraceID().String())
			}
			next.ServeHTTP(w, r)
		})
		return otelhttp.NewHandler(inner, operation,
			otelhttp.WithFilter(func(r *http.Request) bool { return !isProbe(r.URL.Path) }),
			otelhttp.WithPublicEndpointFn(func(*http.Request) bool { return public }),
		)
	}
}

func isProbe(path string) bool {
	return path == "/healthz" || path == "/readyz" || path == "/metrics"
}

func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.ErrorContext(r.Context(), "panic", "panic", rec, "stack", string(debug.Stack()))
					WriteError(w, ErrInternal)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func Observe(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			meta := &requestMeta{}
			outer := r
			r = r.WithContext(context.WithValue(r.Context(), metaKey{}, meta))
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)
			outer.Pattern = r.Pattern
			if sw.status == 0 {
				sw.status = http.StatusOK
			}
			attrs, audit, failed := meta.snapshot()
			route := meta.route()
			if route == "" {
				route = r.Pattern
			}
			if route == "" {
				route = "unmatched"
			}
			elapsed := time.Since(start)
			requestsTotal.WithLabelValues(r.Method, route, statusLabel(sw.status)).Inc()
			observeDuration(r, route, elapsed)
			if isProbe(r.URL.Path) {
				return
			}
			if route != "unmatched" {
				trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(routePath(route)))
			}
			fields := append([]any{
				"method", r.Method,
				"path", r.URL.Path,
				"route", route,
				"status", sw.status,
				"duration_ms", elapsed.Milliseconds(),
				"bytes", sw.bytes,
			}, attrsToArgs(attrs)...)
			level := slog.LevelInfo
			if failed {
				level = slog.LevelError
			}
			log.Log(r.Context(), level, "request", fields...)
			if audit {
				log.InfoContext(r.Context(), "audit", fields...)
			}
		})
	}
}

func routePath(pattern string) string {
	if _, path, ok := strings.Cut(pattern, " "); ok {
		return path
	}
	return pattern
}

func observeDuration(r *http.Request, route string, elapsed time.Duration) {
	observer := requestDuration.WithLabelValues(r.Method, route)
	sc := trace.SpanContextFromContext(r.Context())
	if eo, ok := observer.(prometheus.ExemplarObserver); ok && sc.IsSampled() {
		eo.ObserveWithExemplar(elapsed.Seconds(), prometheus.Labels{"trace_id": sc.TraceID().String()})
		return
	}
	observer.Observe(elapsed.Seconds())
}

func statusLabel(code int) string {
	return strconv.Itoa(code/100) + "xx"
}
