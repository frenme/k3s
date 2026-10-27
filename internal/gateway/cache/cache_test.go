package cache

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

func setup(t *testing.T, status *atomic.Int32, access policy.Access) (http.Handler, *atomic.Int32, *Cache) {
	t.Helper()
	var hits atomic.Int32
	c := New(10)
	table := policy.Table{{Method: "GET", Path: "/api/v1/common/app-config", Access: access, CacheTTL: time.Minute}}
	upstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "must-not-be-cached")
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	return httpx.Chain(upstream, policy.Resolve(table), c.Middleware()), &hits, c
}

func get(h http.Handler, path string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServesRepeatedGetsFromCache(t *testing.T) {
	status := &atomic.Int32{}
	status.Store(200)
	h, hits, _ := setup(t, status, policy.AccessPublic)

	first, second := get(h, "/api/v1/common/app-config"), get(h, "/api/v1/common/app-config")
	if hits.Load() != 1 || first.Header().Get("X-Cache") != "MISS" || second.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("hits %d, %q then %q", hits.Load(), first.Header().Get("X-Cache"), second.Header().Get("X-Cache"))
	}
	if second.Body.String() != `{"items":[]}` || second.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("body %q type %q", second.Body.String(), second.Header().Get("Content-Type"))
	}
	if second.Header().Get("Set-Cookie") != "" {
		t.Fatal("a cookie was cached and replayed to another client")
	}
	if second.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Fatalf("Cache-Control %q", second.Header().Get("Cache-Control"))
	}
	if get(h, "/api/v1/common/app-config?x=1").Header().Get("X-Cache") != "MISS" {
		t.Fatal("different query must be a different entry")
	}
}

func TestConditionalRequestGets304(t *testing.T) {
	status := &atomic.Int32{}
	status.Store(200)
	h, _, _ := setup(t, status, policy.AccessPublic)
	etag := get(h, "/api/v1/common/app-config").Header().Get("ETag")
	rec := get(h, "/api/v1/common/app-config", "If-None-Match", etag)
	if etag == "" || rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Fatalf("etag %q status %d", etag, rec.Code)
	}
}

func TestExpiredEntryIsReloaded(t *testing.T) {
	status := &atomic.Int32{}
	status.Store(200)
	h, hits, c := setup(t, status, policy.AccessPublic)
	now := time.Now()
	c.now = func() time.Time { return now }
	get(h, "/api/v1/common/app-config")
	now = now.Add(2 * time.Minute)
	get(h, "/api/v1/common/app-config")
	if hits.Load() != 2 {
		t.Fatalf("hits %d", hits.Load())
	}
}

func TestServesStaleWhenTheServiceFails(t *testing.T) {
	status := &atomic.Int32{}
	status.Store(200)
	h, _, c := setup(t, status, policy.AccessPublic)
	now := time.Now()
	c.now = func() time.Time { return now }
	get(h, "/api/v1/common/app-config")

	now = now.Add(5 * time.Minute)
	status.Store(503)
	rec := get(h, "/api/v1/common/app-config")
	if rec.Code != http.StatusOK || rec.Header().Get("X-Cache") != "STALE" {
		t.Fatalf("status %d cache %q", rec.Code, rec.Header().Get("X-Cache"))
	}

	now = now.Add(time.Hour)
	if rec := get(h, "/api/v1/common/app-config"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a too old entry must not be served: %d", rec.Code)
	}
}

func TestErrorsAreNotCached(t *testing.T) {
	status := &atomic.Int32{}
	status.Store(500)
	h, hits, _ := setup(t, status, policy.AccessPublic)
	get(h, "/api/v1/common/app-config")
	get(h, "/api/v1/common/app-config")
	if hits.Load() != 2 {
		t.Fatalf("hits %d", hits.Load())
	}
}

func TestOnlyPublicGetsAreCached(t *testing.T) {
	status := &atomic.Int32{}
	status.Store(200)
	h, hits, _ := setup(t, status, policy.AccessUser)
	get(h, "/api/v1/common/app-config")
	get(h, "/api/v1/common/app-config")
	if hits.Load() != 2 {
		t.Fatalf("an authenticated route was cached: %d", hits.Load())
	}
}

func TestConcurrentMissesAreCoalesced(t *testing.T) {
	var hits atomic.Int32
	release := make(chan struct{})
	c := New(10)
	table := policy.Table{{Method: "GET", Path: "/api/v1/common/app-config", Access: policy.AccessPublic, CacheTTL: time.Minute}}
	h := httpx.Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-release
		_, _ = w.Write([]byte("ok"))
	}), policy.Resolve(table), c.Middleware())

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { get(h, "/api/v1/common/app-config") })
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	if hits.Load() != 1 {
		t.Fatalf("upstream hit %d times for one cold key", hits.Load())
	}
}

func TestCacheIsBounded(t *testing.T) {
	status := &atomic.Int32{}
	status.Store(200)
	h, _, c := setup(t, status, policy.AccessPublic)
	for i := range 30 {
		get(h, "/api/v1/common/app-config?i="+strconvItoa(i))
	}
	if len(c.entries) > 10 {
		t.Fatalf("%d entries", len(c.entries))
	}
}

func strconvItoa(i int) string {
	return string(rune('a'+i%26)) + string(rune('a'+i/26))
}
