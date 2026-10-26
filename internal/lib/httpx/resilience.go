package httpx

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ErrCircuitOpen  = errors.New("circuit breaker is open")
	ErrBulkheadFull = errors.New("too many concurrent requests")
)

var (
	clientRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_retries_total",
		Help: "Outgoing HTTP request retries.",
	}, []string{"client"})
	clientRejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_client_rejected_total",
		Help: "Outgoing HTTP requests rejected before being sent.",
	}, []string{"client", "reason"})
	clientCircuitOpen = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "http_client_circuit_open",
		Help: "1 while the circuit breaker of the client is open.",
	}, []string{"client"})
)

type Resilience struct {
	Retries          int
	Backoff          time.Duration
	FailureThreshold int
	Cooldown         time.Duration
	MaxConcurrent    int
}

func DefaultResilience() Resilience {
	return Resilience{
		Retries:          2,
		Backoff:          100 * time.Millisecond,
		FailureThreshold: 5,
		Cooldown:         10 * time.Second,
		MaxConcurrent:    100,
	}
}

type clientConfig struct {
	resilience *Resilience
}

type ClientOption func(*clientConfig)

func WithResilience(r Resilience) ClientOption {
	return func(c *clientConfig) { c.resilience = &r }
}

func WithoutResilience() ClientOption {
	return func(c *clientConfig) { c.resilience = nil }
}

type breaker struct {
	mu          sync.Mutex
	client      string
	threshold   int
	cooldown    time.Duration
	now         func() time.Time
	failures    int
	openedUntil time.Time
	probing     bool
}

func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.failures < b.threshold {
		return true
	}
	if b.now().Before(b.openedUntil) || b.probing {
		return false
	}
	b.probing = true
	return true
}

func (b *breaker) record(ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
	if ok {
		b.failures = 0
		clientCircuitOpen.WithLabelValues(b.client).Set(0)
		return
	}
	b.failures++
	if b.failures >= b.threshold {
		b.openedUntil = b.now().Add(b.cooldown)
		clientCircuitOpen.WithLabelValues(b.client).Set(1)
	}
}

func (b *breaker) abort() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probing = false
}

type resilientTransport struct {
	client  string
	next    http.RoundTripper
	cfg     Resilience
	breaker *breaker
	slots   chan struct{}
	sleep   func(ctx context.Context, d time.Duration) error
}

func newResilientTransport(client string, next http.RoundTripper, cfg Resilience) *resilientTransport {
	return &resilientTransport{
		client:  client,
		next:    next,
		cfg:     cfg,
		breaker: &breaker{client: client, threshold: max(cfg.FailureThreshold, 1), cooldown: cfg.Cooldown, now: time.Now},
		slots:   make(chan struct{}, max(cfg.MaxConcurrent, 1)),
		sleep:   sleep,
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (t *resilientTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if !t.breaker.allow() {
		clientRejected.WithLabelValues(t.client, "circuit_open").Inc()
		return nil, ErrCircuitOpen
	}
	select {
	case t.slots <- struct{}{}:
		defer func() { <-t.slots }()
	default:
		t.breaker.abort()
		clientRejected.WithLabelValues(t.client, "bulkhead").Inc()
		return nil, ErrBulkheadFull
	}

	resp, err := t.attempts(r)
	switch {
	case err != nil && r.Context().Err() != nil:
		t.breaker.abort()
	default:
		t.breaker.record(err == nil && resp.StatusCode < http.StatusInternalServerError)
	}
	return resp, err
}

func (t *resilientTransport) attempts(r *http.Request) (*http.Response, error) {
	total := 1
	if replayable(r) {
		total += t.cfg.Retries
	}
	var (
		resp *http.Response
		err  error
	)
	for i := range total {
		if i > 0 {
			if sleepErr := t.sleep(r.Context(), backoff(t.cfg.Backoff, i)); sleepErr != nil {
				return resp, err
			}
			next, cloneErr := cloneForRetry(r)
			if cloneErr != nil {
				return resp, err
			}
			drain(resp)
			clientRetries.WithLabelValues(t.client).Inc()
			r = next
		}
		resp, err = t.next.RoundTrip(r)
		if !retryable(r, resp, err) {
			break
		}
	}
	return resp, err
}

func replayable(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return r.Body == nil || r.Body == http.NoBody || r.GetBody != nil
	}
	return false
}

func retryable(r *http.Request, resp *http.Response, err error) bool {
	if err != nil {
		return r.Context().Err() == nil
	}
	switch resp.StatusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func cloneForRetry(r *http.Request) (*http.Request, error) {
	next := r.Clone(r.Context())
	if r.Body != nil && r.Body != http.NoBody {
		body, err := r.GetBody()
		if err != nil {
			return nil, err
		}
		next.Body = body
	}
	return next, nil
}

func drain(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
}

func backoff(base time.Duration, attempt int) time.Duration {
	d := base << (attempt - 1)
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}
