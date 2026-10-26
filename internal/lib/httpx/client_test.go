package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestClientRecordsMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	c := NewClient("probe", time.Second, WithoutResilience())
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got := testutil.ToFloat64(clientRequests.WithLabelValues("probe", http.MethodGet, "4xx")); got != 1 {
		t.Fatalf("4xx counter %v", got)
	}
	srv.Close()
	if _, err := c.Get(srv.URL); err == nil {
		t.Fatal("expected error")
	}
	if got := testutil.ToFloat64(clientRequests.WithLabelValues("probe", http.MethodGet, "error")); got != 1 {
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
