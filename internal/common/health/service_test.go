package health

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestCheckCachesBriefly(t *testing.T) {
	var runs atomic.Int32
	s := NewService(nil, Probe{Name: "redis", Run: func(context.Context) error {
		runs.Add(1)
		return nil
	}})
	first := s.Check(t.Context())
	second := s.Check(t.Context())
	if runs.Load() != 1 || !first.CheckedAt.Equal(second.CheckedAt) || !second.Healthy() {
		t.Fatalf("runs %d", runs.Load())
	}
}
