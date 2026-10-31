package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeTransport struct {
	calls   atomic.Int32
	respond func(n int, r *http.Request) (*http.Response, error)
}

func (f *fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f.respond(int(f.calls.Add(1)), r)
}

func reply(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}
}

func newTestRetry(next http.RoundTripper) *retryTransport {
	t := newRetryTransport("test", next, defaultRetries)
	t.wait = func(context.Context, time.Duration) error { return nil }
	return t
}

func send(t *testing.T, rt http.RoundTripper, method string) (*http.Response, error) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), method, "http://svc/x", nil)
	return rt.RoundTrip(req)
}

func TestRetriesIdempotentRequests(t *testing.T) {
	f := &fakeTransport{respond: func(n int, _ *http.Request) (*http.Response, error) {
		switch n {
		case 1:
			return nil, errors.New("connection reset")
		case 2:
			return reply(http.StatusServiceUnavailable), nil
		}
		return reply(http.StatusOK), nil
	}}
	resp, err := send(t, newTestRetry(f), http.MethodGet)
	if err != nil || resp.StatusCode != http.StatusOK || f.calls.Load() != 3 {
		t.Fatalf("resp %v err %v calls %d", resp, err, f.calls.Load())
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
		f := &fakeTransport{respond: func(int, *http.Request) (*http.Response, error) { return reply(tc.status), nil }}
		resp, err := send(t, newTestRetry(f), tc.method)
		if err != nil || resp.StatusCode != tc.status || f.calls.Load() != 1 {
			t.Errorf("%s: resp %v err %v calls %d", name, resp, err, f.calls.Load())
		}
	}
}

func TestGivesUpAfterRetries(t *testing.T) {
	f := &fakeTransport{respond: func(int, *http.Request) (*http.Response, error) { return reply(http.StatusBadGateway), nil }}
	resp, err := send(t, newTestRetry(f), http.MethodGet)
	if err != nil || resp.StatusCode != http.StatusBadGateway || f.calls.Load() != 3 {
		t.Fatalf("resp %v err %v calls %d", resp, err, f.calls.Load())
	}
}

func TestCanceledRequestIsNotRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	f := &fakeTransport{respond: func(int, *http.Request) (*http.Response, error) {
		cancel()
		return nil, context.Canceled
	}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://svc/x", nil)
	if _, err := newTestRetry(f).RoundTrip(req); err == nil || f.calls.Load() != 1 {
		t.Fatalf("err %v calls %d", err, f.calls.Load())
	}
}

func TestWithoutRetriesSendsOneRequest(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	resp, err := NewClient("single", time.Second, WithoutRetries()).Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("calls %d", calls.Load())
	}
}
