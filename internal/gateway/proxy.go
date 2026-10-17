package gateway

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/egor/k3s-base/internal/platform/httpx"
)

func ParseRoutes(spec string) (map[string]*url.URL, error) {
	routes := map[string]*url.URL{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, raw, ok := strings.Cut(part, "=")
		if !ok || name == "" || raw == "" {
			return nil, fmt.Errorf("invalid route %q, expected name=url", part)
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid upstream url %q", raw)
		}
		routes[name] = u
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("no routes configured")
	}
	return routes, nil
}

func newTransport(timeout time.Duration) *http.Transport {
	return &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ResponseHeaderTimeout: timeout,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       60 * time.Second,
	}
}

func NewProxy(routes map[string]*url.URL, log *slog.Logger, upstreamTimeout time.Duration) http.Handler {
	transport := newTransport(upstreamTimeout)
	proxies := make(map[string]*httputil.ReverseProxy, len(routes))
	for name, target := range routes {
		name := name
		proxies[name] = &httputil.ReverseProxy{
			Transport: transport,
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				pr.Out.URL.Path = "/" + strings.TrimPrefix(pr.In.URL.Path, "/api/")
				pr.Out.URL.RawPath = ""
				if prior := pr.In.Header.Values("X-Forwarded-For"); len(prior) > 0 {
					pr.Out.Header["X-Forwarded-For"] = prior
				}
				pr.SetXForwarded()
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Error("upstream error", "upstream", name, "err", err, "request_id", httpx.RequestIDFrom(r.Context()))
				httpx.WriteError(w, http.StatusBadGateway, "upstream unavailable")
			},
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/api/"), "/")
		p, ok := proxies[name]
		if !ok {
			httpx.WriteError(w, http.StatusNotFound, "unknown route")
			return
		}
		p.ServeHTTP(w, r)
	})
}
