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

func TestInt(t *testing.T) {
	t.Setenv("CFG_INT", "42")
	if got := Int("CFG_INT", 1); got != 42 {
		t.Fatalf("got %d", got)
	}
	t.Setenv("CFG_INT_BAD", "x")
	if got := Int("CFG_INT_BAD", 7); got != 7 {
		t.Fatalf("got %d", got)
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
}

func TestInt32(t *testing.T) {
	t.Setenv("CFG_I32", "20")
	if got := Int32("CFG_I32", 1); got != 20 {
		t.Fatalf("got %d", got)
	}
	t.Setenv("CFG_I32_BIG", "9999999999")
	if got := Int32("CFG_I32_BIG", 7); got != 7 {
		t.Fatalf("got %d", got)
	}
}
