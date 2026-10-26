package httpx

import (
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var (
	clientRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_requests_total",
		Help: "Outgoing HTTP requests.",
	}, []string{"client", "method", "code"})
	clientDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_client_request_duration_seconds",
		Help:    "Outgoing HTTP request latency.",
		Buckets: prometheus.DefBuckets,
	}, []string{"client", "method"})
)

type metricsTransport struct {
	client string
	next   http.RoundTripper
}

func (t metricsTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.next.RoundTrip(r)
	code := "error"
	if err == nil {
		code = statusLabel(resp.StatusCode)
	}
	clientRequests.WithLabelValues(t.client, r.Method, code).Inc()
	clientDuration.WithLabelValues(t.client, r.Method).Observe(time.Since(start).Seconds())
	return resp, err
}

func NewClient(name string, timeout time.Duration, opts ...ClientOption) *http.Client {
	cfg := clientConfig{}
	r := DefaultResilience()
	cfg.resilience = &r
	for _, opt := range opts {
		opt(&cfg)
	}
	var transport http.RoundTripper = metricsTransport{client: name, next: tunedTransport()}
	if cfg.resilience != nil {
		transport = newResilientTransport(name, transport, *cfg.resilience)
	}
	return &http.Client{Timeout: timeout, Transport: otelhttp.NewTransport(transport)}
}

func tunedTransport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	t.TLSHandshakeTimeout = 5 * time.Second
	t.MaxIdleConns = 200
	t.MaxIdleConnsPerHost = 100
	t.IdleConnTimeout = 60 * time.Second
	return t
}
