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

