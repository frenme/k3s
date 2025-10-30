package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

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

func NewProxy(routes map[string]*url.URL, log *slog.Logger) http.Handler {
	proxies := make(map[string]*httputil.ReverseProxy, len(routes))
	for name, target := range routes {
		name := name
		proxies[name] = &httputil.ReverseProxy{
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(target)
				pr.Out.URL.Path = "/" + strings.TrimPrefix(strings.TrimPrefix(pr.In.URL.Path, "/api/"), "")
				pr.Out.URL.RawPath = ""
				pr.SetXForwarded()
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Error("upstream error", "upstream", name, "err", err, "request_id", httpx.RequestIDFrom(r.Context()))
				httpx.WriteError(w, http.StatusBadGateway, "upstream unavailable")
			},
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/")
		name, _, _ := strings.Cut(rest, "/")
		p, ok := proxies[name]
		if !ok {
			httpx.WriteError(w, http.StatusNotFound, "unknown route")
			return
		}
		p.ServeHTTP(w, r)
	})
}
