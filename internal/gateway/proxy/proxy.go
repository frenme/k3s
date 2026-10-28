package proxy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egor/k3s-base/internal/gateway/edge"
	"github.com/egor/k3s-base/internal/gateway/model"
	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Options struct {
	Routes         map[string]*url.URL
	Log            *slog.Logger
	DefaultTimeout time.Duration
	Guard          GuardConfig
}

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

func New(opts Options) http.Handler {
	proxies := make(map[string]*httputil.ReverseProxy, len(opts.Routes))
	for name, target := range opts.Routes {
		transport := newGuard(name, httpx.NewTransport("gateway-"+name), opts.Guard)
		proxies[name] = &httputil.ReverseProxy{
			Transport: transport,
			Rewrite:   rewrite(target),
			ModifyResponse: func(resp *http.Response) error {
				resp.Header.Del(httpx.RequestIDHeader)
				return nil
			},
			ErrorHandler: errorHandler(name, opts.Log, opts.Guard.Cooldown),
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version, rest, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/api/"), "/")
		name, _, _ := strings.Cut(rest, "/")
		p, ok := proxies[name]
		if !ok || !isVersion(version) || strings.Contains("/"+rest+"/", "/internal/") {
			httpx.WriteError(w, model.ErrUnknownRoute)
			return
		}
		httpx.LogAttrs(r.Context(), slog.String("upstream", name))

		timeout := policy.From(r.Context()).Rule.Timeout
		if timeout <= 0 {
			timeout = opts.DefaultTimeout
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		p.ServeHTTP(w, r.WithContext(ctx))
	})
}

func rewrite(target *url.URL) func(*httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)
		pr.Out.URL.Path = "/" + strings.TrimPrefix(pr.In.URL.Path, "/api/")
		pr.Out.URL.RawPath = ""
		pr.Out.Header.Set("X-Forwarded-For", edge.IPFrom(pr.In.Context()))
		pr.Out.Header.Set("X-Forwarded-Host", pr.In.Host)
		pr.Out.Header.Set("X-Forwarded-Proto", forwardedProto(pr.In))
	}
}

func forwardedProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
		return proto
	}
	return "http"
}

func errorHandler(upstream string, log *slog.Logger, cooldown time.Duration) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge):
			httpx.WriteError(w, model.ErrPayloadTooLarge)
		case errors.Is(err, ErrCircuitOpen):
			w.Header().Set("Retry-After", strconv.Itoa(max(int(math.Ceil(cooldown.Seconds())), 1)))
			httpx.WriteError(w, model.ErrUpstreamOpen)
		case errors.Is(err, ErrOverloaded):
			w.Header().Set("Retry-After", "1")
			httpx.WriteError(w, model.ErrUpstreamOverload)
		case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
			log.WarnContext(r.Context(), "upstream timeout", "upstream", upstream, "err", err)
			httpx.WriteError(w, model.ErrUpstreamTimeout)
		case errors.Is(err, context.Canceled):
			w.WriteHeader(499)
		default:
			log.ErrorContext(r.Context(), "upstream error", "upstream", upstream, "err", err)
			httpx.WriteError(w, httpx.ErrUnavailable)
		}
	}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func isVersion(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
