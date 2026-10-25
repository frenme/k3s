package config

import (
	"testing"
	"time"
)

func TestString(t *testing.T) {
	t.Setenv("CFG_STR", "value")
	if got := String("CFG_STR", "def"); got != "value" {
		t.Fatalf("got %q", got)
	}
	if got := String("CFG_STR_MISSING", "def"); got != "def" {
		t.Fatalf("got %q", got)
	}
}

func TestDuration(t *testing.T) {
	t.Setenv("CFG_DUR", "3s")
	if got := Duration("CFG_DUR", time.Second); got != 3*time.Second {
		t.Fatalf("got %s", got)
	}
	if got := Duration("CFG_DUR_MISSING", time.Second); got != time.Second {
		t.Fatalf("got %s", got)
	}
	t.Setenv("CFG_DUR_BAD", "5")
	expectPanic(t, func() { Duration("CFG_DUR_BAD", time.Second) })
}

func TestInt32(t *testing.T) {
	t.Setenv("CFG_I32", "20")
	if got := Int32("CFG_I32", 1); got != 20 {
		t.Fatalf("got %d", got)
	}
	t.Setenv("CFG_I32_BIG", "9999999999")
	expectPanic(t, func() { Int32("CFG_I32_BIG", 7) })
}

func expectPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}
