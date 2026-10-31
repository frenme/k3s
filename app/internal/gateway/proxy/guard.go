package proxy

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ErrCircuitOpen = errors.New("circuit breaker is open")
	ErrOverloaded  = errors.New("too many concurrent requests to the service")
)

var (
	rejected = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_upstream_rejected_total",
		Help: "Requests the gateway refused to send to a service.",
	}, []string{"upstream", "reason"})
	circuitOpen = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gateway_upstream_circuit_open",
		Help: "1 while the circuit breaker of the service is open.",
	}, []string{"upstream"})
)

type GuardConfig struct {
	FailureThreshold int
	Cooldown         time.Duration
	MaxConcurrent    int
}

type guard struct {
	name  string
	next  http.RoundTripper
	slots chan struct{}
	now   func() time.Time

	mu          sync.Mutex
	threshold   int
	cooldown    time.Duration
	failures    int
	openedUntil time.Time
	probing     bool
}

func (g *guard) RoundTrip(r *http.Request) (*http.Response, error) {
	if !g.allow() {
		rejected.WithLabelValues(g.name, "circuit_open").Inc()
		return nil, ErrCircuitOpen
	}
	select {
	case g.slots <- struct{}{}:
		defer func() { <-g.slots }()
	default:
		g.abort()
		rejected.WithLabelValues(g.name, "overloaded").Inc()
		return nil, ErrOverloaded
	}

	resp, err := g.next.RoundTrip(r)
	switch {
	case err != nil && r.Context().Err() != nil:
		g.abort()
	default:
		g.record(err == nil && resp.StatusCode < http.StatusInternalServerError)
	}
	return resp, err
}

func (g *guard) allow() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.failures < g.threshold {
		return true
	}
	if g.now().Before(g.openedUntil) || g.probing {
		return false
	}
	g.probing = true
	return true
}

func (g *guard) record(ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.probing = false
	if ok {
		g.failures = 0
		circuitOpen.WithLabelValues(g.name).Set(0)
		return
	}
	g.failures++
	if g.failures >= g.threshold {
		g.openedUntil = g.now().Add(g.cooldown)
		circuitOpen.WithLabelValues(g.name).Set(1)
	}
}

func (g *guard) abort() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.probing = false
}

func newGuard(name string, next http.RoundTripper, cfg GuardConfig) *guard {
	return &guard{
		name:      name,
		next:      next,
		slots:     make(chan struct{}, max(cfg.MaxConcurrent, 1)),
		now:       time.Now,
		threshold: max(cfg.FailureThreshold, 1),
		cooldown:  cfg.Cooldown,
	}
}
