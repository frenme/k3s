package edge

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

type ClientIP struct {
	trusted []netip.Prefix
}

type clientIPKey struct{}

func NewClientIP(cidrs []string) (*ClientIP, error) {
	c := &ClientIP{}
	for _, raw := range cidrs {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: %w", raw, err)
		}
		c.trusted = append(c.trusted, prefix)
	}
	return c, nil
}

func (c *ClientIP) Middleware() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := c.Resolve(r)
			httpx.LogAttrs(r.Context(), slog.String("client_ip", ip))
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, ip)))
		})
	}
}

func (c *ClientIP) Resolve(r *http.Request) string {
	peer := peerAddr(r.RemoteAddr)
	if !peer.IsValid() || !c.isTrusted(peer) {
		return peer.String()
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return peer.String()
		}
		if !c.isTrusted(addr) {
			return addr.String()
		}
	}
	return peer.String()
}

func (c *ClientIP) isTrusted(addr netip.Addr) bool {
	addr = addr.Unmap()
	for _, p := range c.trusted {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func peerAddr(remote string) netip.Addr {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func IPFrom(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}
