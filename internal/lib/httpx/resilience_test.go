package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type stub struct {
	calls   atomic.Int32
	respond func(n int, r *http.Request) (*http.Response, error)
}

func (s *stub) RoundTrip(r *http.Request) (*http.Response, error) {
	return s.respond(int(s.calls.Add(1)), r)
}

func reply(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}
}

func newTransport(next http.RoundTripper, cfg Resilience) *resilientTransport {
	t := newResilientTransport("test", next, cfg)
	t.sleep = func(context.Context, time.Duration) error { return nil }
	return t
}

func do(t *testing.T, rt http.RoundTripper, method string) (*http.Response, error) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), method, "http://svc/x", nil)
	return rt.RoundTrip(req)
}

func TestRetriesIdempotentRequests(t *testing.T) {
	s := &stub{respond: func(n int, _ *http.Request) (*http.Response, error) {
		switch n {
		case 1:
			return nil, errors.New("connection reset")
		case 2:
			return reply(http.StatusServiceUnavailable), nil
		}
		return reply(http.StatusOK), nil
	}}
	resp, err := do(t, newTransport(s, DefaultResilience()), http.MethodGet)
	if err != nil || resp.StatusCode != http.StatusOK || s.calls.Load() != 3 {
		t.Fatalf("resp %v err %v calls %d", resp, err, s.calls.Load())
	}
}

func TestDoesNotRetryUnsafeOrDefinitiveResponses(t *testing.T) {
	for name, tc := range map[string]struct {
		method string
		status int
	}{
		"post 503":   {http.MethodPost, http.StatusServiceUnavailable},
		"get 404":    {http.MethodGet, http.StatusNotFound},
		"get 500":    {http.MethodGet, http.StatusInternalServerError},
		"get 422":    {http.MethodGet, http.StatusUnprocessableEntity},
		"delete 503": {http.MethodDelete, http.StatusServiceUnavailable},
	} {
		s := &stub{respond: func(int, *http.Request) (*http.Response, error) { return reply(tc.status), nil }}
		resp, err := do(t, newTransport(s, DefaultResilience()), tc.method)
		if err != nil || resp.StatusCode != tc.status || s.calls.Load() != 1 {
			t.Errorf("%s: resp %v err %v calls %d", name, resp, err, s.calls.Load())
		}
	}
}

func TestGivesUpAfterRetries(t *testing.T) {
	s := &stub{respond: func(int, *http.Request) (*http.Response, error) { return reply(http.StatusBadGateway), nil }}
	resp, err := do(t, newTransport(s, DefaultResilience()), http.MethodGet)
	if err != nil || resp.StatusCode != http.StatusBadGateway || s.calls.Load() != 3 {
		t.Fatalf("resp %v err %v calls %d", resp, err, s.calls.Load())
	}
}

func TestCanceledRequestIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	s := &stub{respond: func(int, *http.Request) (*http.Response, error) {
		cancel()
		return nil, context.Canceled
	}}
	tr := newTransport(s, DefaultResilience())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://svc/x", nil)
	if _, err := tr.RoundTrip(req); err == nil || s.calls.Load() != 1 {
		t.Fatalf("err %v calls %d", err, s.calls.Load())
	}
	if tr.breaker.failures != 0 {
		t.Fatal("a canceled request counted as a failure")
	}
}

func TestCircuitOpensAndRecovers(t *testing.T) {
	healthy := false
	s := &stub{respond: func(int, *http.Request) (*http.Response, error) {
		if healthy {
			return reply(http.StatusOK), nil
		}
		return reply(http.StatusInternalServerError), nil
	}}
	cfg := DefaultResilience()
	cfg.FailureThreshold = 3
	tr := newTransport(s, cfg)
	now := time.Now()
	tr.breaker.now = func() time.Time { return now }

	for range 3 {
		if _, err := do(t, tr, http.MethodGet); err != nil {
			t.Fatal(err)
		}
	}
	calls := s.calls.Load()
	if _, err := do(t, tr, http.MethodGet); !errors.Is(err, ErrCircuitOpen) || s.calls.Load() != calls {
		t.Fatalf("expected a fast failure, err %v calls %d", err, s.calls.Load())
	}

	now = now.Add(cfg.Cooldown + time.Second)
	healthy = true
	if resp, err := do(t, tr, http.MethodGet); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("probe: %v %v", resp, err)
	}
	if _, err := do(t, tr, http.MethodGet); err != nil {
		t.Fatalf("circuit stayed open after a successful probe: %v", err)
	}
}

func TestHalfOpenAllowsOneProbe(t *testing.T) {
	cfg := DefaultResilience()
	cfg.FailureThreshold = 1
	b := &breaker{client: "t", threshold: cfg.FailureThreshold, cooldown: time.Second, now: time.Now}
	b.record(false)
	b.openedUntil = time.Now().Add(-time.Second)
	if !b.allow() || b.allow() {
		t.Fatal("half-open must let exactly one request through")
	}
	b.record(false)
	if b.allow() {
		t.Fatal("a failed probe must reopen the circuit")
	}
}

func TestBulkheadRejectsOverflow(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 2)
	s := &stub{respond: func(int, *http.Request) (*http.Response, error) {
		entered <- struct{}{}
		<-release
		return reply(http.StatusOK), nil
	}}
	cfg := DefaultResilience()
	cfg.MaxConcurrent = 1
	tr := newTransport(s, cfg)
	done := make(chan error, 1)
	go func() {
		_, err := do(t, tr, http.MethodGet)
		done <- err
	}()
	<-entered
	if _, err := do(t, tr, http.MethodGet); !errors.Is(err, ErrBulkheadFull) {
		t.Fatalf("overflow: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := do(t, tr, http.MethodGet); err != nil {
		t.Fatalf("slot was not released: %v", err)
	}
}

func TestNewClientWithoutResilience(t *testing.T) {
	c := NewClient("plain", time.Second, WithoutResilience())
	if c.Transport == nil {
		t.Fatal("no transport")
	}
}
