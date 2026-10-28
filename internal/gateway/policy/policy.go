package policy

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Rule struct {
	Method   string
	Path     string
	Access   httpx.Access
	Timeout  time.Duration
	MaxBody  int64
	CacheTTL time.Duration
}

type Route struct {
	Rule     Rule
	Template string
	Matched  bool
}

type Table []Rule

var unmatched = Route{Rule: Rule{Access: httpx.AccessUser}, Template: "unmatched"}

var static = Table{
	{Method: http.MethodGet, Path: "/api/v1/*/docs", Access: httpx.AccessPublic},
	{Method: http.MethodGet, Path: "/api/v1/*/openapi.json", Access: httpx.AccessPublic},
	{Method: http.MethodGet, Path: "/api/v1/*/openapi.yaml", Access: httpx.AccessPublic},
	{Path: "/api/v1/admin/**", Access: httpx.AccessAdmin},
}

func Default() Table {
	return slices.Concat(static, generated)
}

func sec(n int) time.Duration {
	return time.Duration(n) * time.Second
}

func (r Rule) methodMatches(method string) bool {
	return r.Method == "" || r.Method == method || (r.Method == http.MethodGet && method == http.MethodHead)
}

func (r Rule) matches(method string, segments []string) bool {
	if !r.methodMatches(method) {
		return false
	}
	want := split(r.Path)
	for i, w := range want {
		if w == "**" {
			return true
		}
		if i >= len(segments) || (w != "*" && w != segments[i]) {
			return false
		}
	}
	return len(want) == len(segments)
}

func (t Table) Find(method, path string) Route {
	segments := split(path)
	for _, rule := range t {
		if rule.matches(method, segments) {
			template := rule.Path
			if rule.Method != "" {
				template = rule.Method + " " + rule.Path
			}
			return Route{Rule: rule, Template: template, Matched: true}
		}
	}
	return unmatched
}

func split(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

type routeKey struct{}

func Resolve(table Table) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := table.Find(r.Method, r.URL.Path)
			httpx.SetRoute(r.Context(), route.Template)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), routeKey{}, route)))
		})
	}
}

func From(ctx context.Context) Route {
	route, ok := ctx.Value(routeKey{}).(Route)
	if !ok {
		return unmatched
	}
	return route
}
