package pagination

import (
	"errors"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	in := Cursor{CreatedAt: time.Date(2026, 10, 26, 19, 0, 0, 123456000, time.UTC), ID: "0b8f6a1e-0000-4000-8000-000000000001"}
	out, err := Decode(in.Encode())
	if err != nil || out == nil || !out.CreatedAt.Equal(in.CreatedAt) || out.ID != in.ID {
		t.Fatalf("%+v %v", out, err)
	}
}

func TestDecodeEmptyMeansFirstPage(t *testing.T) {
	if c, err := Decode(""); c != nil || err != nil {
		t.Fatalf("%v %v", c, err)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	for _, s := range []string{"!!!", "bm9wZQ", "YWJjfGlk"} {
		if _, err := Decode(s); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("%q: %v", s, err)
		}
	}
}

func TestNewPage(t *testing.T) {
	type row struct {
		at time.Time
		id string
	}
	now := time.Now()
	of := func(r row) Cursor { return Cursor{CreatedAt: r.at, ID: r.id} }
	rows := []row{{now, "a"}, {now, "b"}, {now, "c"}}
	if p := NewPage(rows, 3, of); len(p.Items) != 3 || p.Next != "" {
		t.Fatalf("%+v", p)
	}
	p := NewPage(rows, 2, of)
	if len(p.Items) != 2 || p.Next == "" {
		t.Fatalf("%+v", p)
	}
	c, err := Decode(p.Next)
	if err != nil || c.ID != "b" {
		t.Fatalf("%+v %v", c, err)
	}
}
