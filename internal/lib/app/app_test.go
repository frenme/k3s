package app

import (
	"context"
	"errors"
	"testing"
)

func TestCloseRunsInReverseOrder(t *testing.T) {
	s := &Service{}
	var order []int
	s.OnClose(func() { order = append(order, 1) })
	s.OnClose(func() { order = append(order, 2) })
	s.close()
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Fatalf("order %v", order)
	}
}

func TestRunClosesWhenSetupFails(t *testing.T) {
	s := &Service{}
	closed := false
	want := errors.New("setup failed")
	err := s.run(context.Background(), func(ctx context.Context, s *Service) error {
		s.OnClose(func() { closed = true })
		return want
	})
	if !errors.Is(err, want) || !closed {
		t.Fatalf("err %v closed %v", err, closed)
	}
}

func TestCacheDisabledWithoutRedis(t *testing.T) {
	t.Setenv("REDIS_ADDR", "")
	s := &Service{Name: "x"}
	if s.Cache() != nil {
		t.Fatal("expected nil cache")
	}
}
