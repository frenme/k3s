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

