package health

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

func TestCheckCachesBriefly(t *testing.T) {
	var runs atomic.Int32
	s := NewService(nil, map[string]httpx.Check{"redis": func(context.Context) error {
		runs.Add(1)
		return nil
	}})
	first := s.Check(t.Context())
	second := s.Check(t.Context())
	if runs.Load() != 1 || !first.CheckedAt.Equal(second.CheckedAt) || !second.Healthy() {
		t.Fatalf("runs %d", runs.Load())
	}
}
