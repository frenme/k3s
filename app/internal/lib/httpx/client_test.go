package httpx

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestClientRecordsMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	c := NewClient("probe", time.Second, WithoutRetries())
	teapots := clientRequests.WithLabelValues("probe", http.MethodGet, "4xx")
	failures := clientRequests.WithLabelValues("probe", http.MethodGet, "error")
	teapotsBefore, failuresBefore := testutil.ToFloat64(teapots), testutil.ToFloat64(failures)
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := testutil.ToFloat64(teapots) - teapotsBefore; got != 1 {
		t.Fatalf("4xx counter %v", got)
	}
	srv.Close()
	if _, err := c.Get(srv.URL); err == nil {
		t.Fatal("expected error")
	}
	if got := testutil.ToFloat64(failures) - failuresBefore; got != 1 {
		t.Fatalf("error counter %v", got)
	}
}

func TestTransportKeepsConnectionsForOneHost(t *testing.T) {
	a, b := tunedTransport(), tunedTransport()
	if a.MaxIdleConnsPerHost != 100 || a.TLSHandshakeTimeout == 0 || a.Proxy == nil {
		t.Fatalf("%+v", a)
	}
	if a == b {
		t.Fatal("clients must not share a transport")
	}
}

func TestRotatingTransportClosesEveryNthConnection(t *testing.T) {
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()
	c := &http.Client{Transport: &rotatingTransport{next: tunedTransport()}}
	for range rotateEvery * 3 {
		resp, err := c.Get(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	if got := conns.Load(); got != 3 {
		t.Fatalf("connections %d", got)
	}
}
