package lock

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newLockers(t *testing.T) (*Locker, *Locker, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return New(client, "svc", "pod-a"), New(client, "svc", "pod-b"), mr
}

func TestOnlyOneOwnerHoldsTheLock(t *testing.T) {
	a, b, _ := newLockers(t)
	release, err := a.Acquire(t.Context(), "job", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Acquire(t.Context(), "job", time.Minute); !errors.Is(err, ErrHeld) {
		t.Fatalf("err %v", err)
	}
	release()
	if _, err := b.Acquire(t.Context(), "job", time.Minute); err != nil {
		t.Fatalf("after release: %v", err)
	}
}

func TestReleaseDoesNotDropSomeoneElsesLock(t *testing.T) {
	a, b, mr := newLockers(t)
	release, err := a.Acquire(t.Context(), "job", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	mr.FastForward(2 * time.Minute)
	if _, err := b.Acquire(t.Context(), "job", time.Minute); err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := a.Acquire(t.Context(), "job", time.Minute); !errors.Is(err, ErrHeld) {
		t.Fatalf("b's lock was released by a: %v", err)
	}
}

func TestNilWithoutClient(t *testing.T) {
	if New(nil, "svc", "pod") != nil {
		t.Fatal("expected nil locker")
	}
}
