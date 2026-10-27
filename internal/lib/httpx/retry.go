package httpx

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	defaultRetries = 2
	retryBaseDelay = 100 * time.Millisecond
)

var clientRetries = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "http_client_retries_total",
	Help: "Outgoing HTTP request retries.",
}, []string{"client"})

type retryTransport struct {
	client  string
	next    http.RoundTripper
	retries int
	wait    func(ctx context.Context, d time.Duration) error
}

func newRetryTransport(client string, next http.RoundTripper, retries int) *retryTransport {
	return &retryTransport{client: client, next: next, retries: retries, wait: waitFor}
}

func waitFor(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (t *retryTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	attempts := 1
	if safeToRepeat(r) {
		attempts += t.retries
	}
	var (
		resp *http.Response
		err  error
	)
	for i := range attempts {
		if i > 0 {
			if waitErr := t.wait(r.Context(), retryDelay(i)); waitErr != nil {
				return resp, err
			}
			next, cloneErr := cloneForRetry(r)
			if cloneErr != nil {
				return resp, err
			}
			discardBody(resp)
			clientRetries.WithLabelValues(t.client).Inc()
			r = next
		}
		resp, err = t.next.RoundTrip(r)
		if !shouldRetry(r, resp, err) {
			break
		}
	}
	return resp, err
}

func safeToRepeat(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return r.Body == nil || r.Body == http.NoBody || r.GetBody != nil
	}
	return false
}

func shouldRetry(r *http.Request, resp *http.Response, err error) bool {
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

func discardBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	_ = resp.Body.Close()
}

func retryDelay(attempt int) time.Duration {
	d := retryBaseDelay << (attempt - 1)
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}
