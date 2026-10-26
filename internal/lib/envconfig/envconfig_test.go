package envconfig

import (
	"strings"
	"testing"
	"time"
)

type nested struct {
	Timeout time.Duration `env:"NESTED_TIMEOUT" default:"3s"`
}

type sample struct {
	Name    string   `env:"NAME" default:"svc"`
	URL     string   `env:"URL" required:"true"`
	Conns   int32    `env:"CONNS" default:"10"`
	Debug   bool     `env:"DEBUG"`
	Origins []string `env:"ORIGINS"`
	Nested  nested
}

func lookup(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

func TestLoadDefaultsAndValues(t *testing.T) {
	var s sample
	err := LoadFrom(lookup(map[string]string{"URL": "u", "CONNS": "7", "DEBUG": "true", "ORIGINS": "a, b,,c", "NESTED_TIMEOUT": "1m"}), &s)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "svc" || s.URL != "u" || s.Conns != 7 || !s.Debug || len(s.Origins) != 3 || s.Origins[1] != "b" || s.Nested.Timeout != time.Minute {
		t.Fatalf("%+v", s)
	}
	var d sample
	if err := LoadFrom(lookup(map[string]string{"URL": "u"}), &d); err != nil || d.Nested.Timeout != 3*time.Second || d.Conns != 10 {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	var s sample
	err := LoadFrom(lookup(map[string]string{"CONNS": "x", "NESTED_TIMEOUT": "5"}), &s)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"URL is required", "CONNS: invalid integer", "NESTED_TIMEOUT: invalid duration"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing %q in %v", want, err)
		}
	}
}

func TestLoadRejectsNonStruct(t *testing.T) {
	var s string
	if err := LoadFrom(lookup(nil), &s); err == nil {
		t.Fatal("expected error")
	}
}
