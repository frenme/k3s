package auth

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/egor/k3s-base/internal/gateway/model"
	"github.com/egor/k3s-base/internal/gateway/policy"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

const (
	UserIDHeader    = "X-User-ID"
	UserRolesHeader = "X-User-Roles"
	AdminRole       = "admin"
)

func Authorize(verifier *Verifier, roles RoleSource, log *slog.Logger) httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del(UserIDHeader)
			r.Header.Del(UserRolesHeader)
			access := policy.From(r.Context()).Rule.Access
			if access == policy.AccessPublic {
				next.ServeHTTP(w, r)
				return
			}

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
			httpx.LogAttrs(r.Context(), slog.String("user_id", subject))

			if access == policy.AccessAdmin {
				httpx.MarkAudit(r.Context())
				granted, err := roles.Roles(r.Context(), subject)
				if err != nil {
					log.ErrorContext(r.Context(), "role lookup failed", "user_id", subject, "err", err)
					httpx.WriteError(w, model.ErrAuthUnavailable)
					return
				}
				if !slices.Contains(granted, AdminRole) {
					httpx.WriteError(w, model.ErrForbidden)
					return
				}
				r.Header.Set(UserRolesHeader, strings.Join(granted, ","))
			}
			next.ServeHTTP(w, r)
		})
	}
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
	httpx.WriteError(w, model.ErrUnauthorized)
}
