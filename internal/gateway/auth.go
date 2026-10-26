package gateway

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

const (
	adminUser     = "admin"
	adminPassword = "admin-dev-password"

	UserIDHeader = "X-User-ID"
)

type Access int

const (
	AccessUser Access = iota
	AccessPublic
	AccessAdmin
)

type Rule struct {
	Method string
	Path   string
	Access Access
}

var staticRules = []Rule{
	{http.MethodGet, "/api/v1/*/docs", AccessPublic},
	{http.MethodGet, "/api/v1/*/openapi.json", AccessPublic},
	{http.MethodGet, "/api/v1/*/openapi.yaml", AccessPublic},
	{"", "/api/v1/admin/**", AccessAdmin},
}

var DefaultRules = slices.Concat(staticRules, generatedRules)

var (
	errUnauthorized      = httpx.Error{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Valid access token required"}
	errAdminUnauthorized = httpx.Error{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Admin credentials required"}
)

func (r Rule) matches(method string, segments []string) bool {
	if r.Method != "" && r.Method != method {
		return false
	}
	want := strings.Split(strings.Trim(r.Path, "/"), "/")
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

func Authorize(rules []Rule, verifier *Verifier) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del(UserIDHeader)
			switch accessFor(rules, r) {
			case AccessPublic:
				next.ServeHTTP(w, r)
			case AccessAdmin:
				user, password, ok := r.BasicAuth()
				if !ok || subtle.ConstantTimeCompare([]byte(user+":"+password), []byte(adminUser+":"+adminPassword)) != 1 {
					w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
					httpx.WriteError(w, errAdminUnauthorized)
					return
				}
				r.Header.Del("Authorization")
				next.ServeHTTP(w, r)
			default:
				token, ok := bearerToken(r)
				if !ok {
					unauthorized(w)
					return
				}
				subject, err := verifier.Subject(token)
				if err != nil {
					unauthorized(w)
					return
				}
				r.Header.Del("Authorization")
				r.Header.Set(UserIDHeader, subject)
				next.ServeHTTP(w, r)
			}
		})
	}
}

func accessFor(rules []Rule, r *http.Request) Access {
	segments := splitPath(r.URL.Path)
	for _, rule := range rules {
		if rule.matches(r.Method, segments) {
			return rule.Access
		}
	}
	return AccessUser
}

func splitPath(path string) []string {
	return strings.Split(strings.Trim(path, "/"), "/")
}

func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
	httpx.WriteError(w, errUnauthorized)
}
