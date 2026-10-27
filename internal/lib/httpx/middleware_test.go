package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func tracedServer(t *testing.T, public bool) (*tracetest.SpanRecorder, http.Handler) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	mux := http.NewServeMux()
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {})
	return recorder, Chain(mux, Trace("test", public), Observe(discard))
}

func TestTraceSetsRequestIDAndRouteSpan(t *testing.T) {
	recorder, h := tracedServer(t, false)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items/42", nil))

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans %d", len(spans))
	}
	if got := spans[0].Name(); got != "GET /items/{id}" {
		t.Fatalf("span name %q", got)
	}
	route := ""
	for _, a := range spans[0].Attributes() {
		if a.Key == "http.route" {
			route = a.Value.AsString()
		}
	}
	if route != "/items/{id}" {
		t.Fatalf("http.route %q", route)
	}
	if got := rec.Header().Get(RequestIDHeader); got != spans[0].SpanContext().TraceID().String() {
		t.Fatalf("header %q trace %s", got, spans[0].SpanContext().TraceID())
	}
}

func TestTraceContinuesIncomingTrace(t *testing.T) {
	recorder, h := tracedServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if got := recorder.Ended()[0].SpanContext().TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("trace %s", got)
	}
}

func TestTracePublicStartsNewTrace(t *testing.T) {
	recorder, h := tracedServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	h.ServeHTTP(httptest.NewRecorder(), req)
	span := recorder.Ended()[0]
	if span.SpanContext().TraceID().String() == "4bf92f3577b34da6a3ce929d0e0e4736" || span.Parent().IsValid() {
		t.Fatalf("public endpoint continued trace %s", span.SpanContext().TraceID())
	}
	if len(span.Links()) != 1 {
		t.Fatalf("links %d", len(span.Links()))
	}
}

func TestTraceSkipsProbes(t *testing.T) {
	recorder, h := tracedServer(t, false)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if len(recorder.Ended()) != 0 || rec.Header().Get(RequestIDHeader) != "" {
		t.Fatalf("probe traced: %d spans", len(recorder.Ended()))
	}
}

func TestObserveRecordsExemplar(t *testing.T) {
	_, h := tracedServer(t, false)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items/7", nil))
	traceID := rec.Header().Get(RequestIDHeader)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != "http_request_duration_seconds" {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, b := range m.GetHistogram().GetBucket() {
				if ex := b.GetExemplar(); ex != nil {
					for _, l := range ex.GetLabel() {
						if l.GetName() == "trace_id" && l.GetValue() == traceID {
							return
						}
					}
				}
			}
		}
	}
	t.Fatalf("no exemplar with trace_id %s", traceID)
}

func TestRecover(t *testing.T) {
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}), Recover(discard))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestObserveKeepsStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	h := Chain(mux, Observe(discard))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestRoutePath(t *testing.T) {
	cases := map[string]string{"GET /items/{id}": "/items/{id}", "/api/": "/api/"}
	for pattern, want := range cases {
		if got := routePath(pattern); got != want {
			t.Fatalf("%s: got %s", pattern, got)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	cases := map[int]string{200: "2xx", 404: "4xx", 503: "5xx"}
	for code, want := range cases {
		if got := statusLabel(code); got != want {
			t.Fatalf("%d: got %s", code, got)
		}
	}
}

func TestObserveCountsPanics(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	h := Chain(mux, Observe(discard), Recover(discard))
	before := testutil.ToFloat64(requestsTotal.WithLabelValues("GET", "GET /boom", "5xx"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))
	after := testutil.ToFloat64(requestsTotal.WithLabelValues("GET", "GET /boom", "5xx"))
	if rec.Code != http.StatusInternalServerError || after != before+1 {
		t.Fatalf("status %d counter %v -> %v", rec.Code, before, after)
	}
}

func TestObserveUsesTheRouteSetByHandlers(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		SetRoute(r.Context(), "GET /api/v1/orders/*")
		LogAttrs(r.Context(), slog.String("user_id", "u-1"))
		MarkAudit(r.Context())
		_, _ = w.Write([]byte("hello"))
	})
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	h := Chain(mux, Observe(log))

	counted := testutil.ToFloat64(requestsTotal.WithLabelValues("GET", "GET /api/v1/orders/*", "2xx"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/orders/42", nil))
	if got := testutil.ToFloat64(requestsTotal.WithLabelValues("GET", "GET /api/v1/orders/*", "2xx")); got != counted+1 {
		t.Fatalf("the request was counted under another route: %v -> %v", counted, got)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected a request and an audit line, got %d: %s", len(lines), buf.String())
	}
	for i, msg := range []string{"request", "audit"} {
		var rec map[string]any
		if err := json.Unmarshal([]byte(lines[i]), &rec); err != nil {
			t.Fatal(err)
		}
		if rec["msg"] != msg || rec["route"] != "GET /api/v1/orders/*" || rec["user_id"] != "u-1" || rec["bytes"] != float64(5) || rec["status"] != float64(200) {
			t.Fatalf("%s line: %v", msg, rec)
		}
	}
}
