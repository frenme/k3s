package pg

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryStopsOnSuccess(t *testing.T) {
	calls := 0
	err := retry(context.Background(), func() error {
		calls++
		if calls < 2 {
			return errors.New("not yet")
		}
		return nil
	}, func(error) bool { return true })
	if err != nil || calls != 2 {
		t.Fatalf("err %v calls %d", err, calls)
	}
}

func TestRetryStopsOnPermanentError(t *testing.T) {
	permanent := errors.New("permanent")
	calls := 0
	err := retry(context.Background(), func() error {
		calls++
		return permanent
	}, func(error) bool { return false })
	if !errors.Is(err, permanent) || calls != 1 {
		t.Fatalf("err %v calls %d", err, calls)
	}
}

func TestRetryStopsOnContextDone(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := retry(ctx, func() error { return errors.New("down") }, func(error) bool { return true })
	if err == nil {
		t.Fatal("expected error")
	}
}
