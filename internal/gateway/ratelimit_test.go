package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/egor/k3s-base/internal/platform/rds"
)

func TestLimiterBlocksAfterLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	l := NewLimiter(rds.New(mr.Addr()), 3, time.Minute, 1)
	h := l.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	codes := []int{}
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "9.9.9.9")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	want := []int{200, 200, 200, 429, 429}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("codes %v", codes)
		}
	}
}

func TestLimiterIsPerClient(t *testing.T) {
	mr := miniredis.RunT(t)
	l := NewLimiter(rds.New(mr.Addr()), 1, time.Minute, 1)
	h := l.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", ip)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", ip, rec.Code)
		}
	}
}

func TestLimiterFailsOpen(t *testing.T) {
	mr := miniredis.RunT(t)
	l := NewLimiter(rds.New(mr.Addr()), 1, time.Minute, 1)
	mr.Close()
	h := l.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	if got := clientIP(req, 1); got != "10.0.0.1" {
		t.Fatalf("got %s", got)
	}
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 5.5.5.5")
	if got := clientIP(req, 1); got != "5.5.5.5" {
		t.Fatalf("got %s", got)
	}
	if got := clientIP(req, 2); got != "6.6.6.6" {
		t.Fatalf("got %s", got)
	}
	if got := clientIP(req, 0); got != "10.0.0.1" {
		t.Fatalf("got %s", got)
	}
	if got := clientIP(req, 5); got != "10.0.0.1" {
		t.Fatalf("got %s", got)
	}
}

func TestClientIPSpoofedFirstEntryIgnored(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 7.7.7.7")
	if got := clientIP(req, 1); got != "7.7.7.7" {
		t.Fatalf("got %s", got)
	}
}
