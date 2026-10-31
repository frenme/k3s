package httpx

import (
	"context"
	"log/slog"
	"sync"
)

type metaKey struct{}

type requestMeta struct {
	mu       sync.Mutex
	routeTpl string
	attrs    []slog.Attr
	audit    bool
	failed   bool
}

func (m *requestMeta) route() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.routeTpl
}

func (m *requestMeta) snapshot() (attrs []slog.Attr, audit, failed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]slog.Attr(nil), m.attrs...), m.audit, m.failed
}

func metaFrom(ctx context.Context) *requestMeta {
	m, _ := ctx.Value(metaKey{}).(*requestMeta)
	return m
}

func SetRoute(ctx context.Context, route string) {
	if m := metaFrom(ctx); m != nil {
		m.mu.Lock()
		m.routeTpl = route
		m.mu.Unlock()
	}
}

func LogAttrs(ctx context.Context, attrs ...slog.Attr) {
	if m := metaFrom(ctx); m != nil {
		m.mu.Lock()
		m.attrs = append(m.attrs, attrs...)
		m.mu.Unlock()
	}
}

func MarkAudit(ctx context.Context) {
	if m := metaFrom(ctx); m != nil {
		m.mu.Lock()
		m.audit = true
		m.mu.Unlock()
	}
}

func markFailed(ctx context.Context) {
	if m := metaFrom(ctx); m != nil {
		m.mu.Lock()
		m.failed = true
		m.mu.Unlock()
	}
}

func attrsToArgs(attrs []slog.Attr) []any {
	out := make([]any, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, a)
	}
	return out
}
