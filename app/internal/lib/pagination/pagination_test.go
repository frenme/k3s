package pagination

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
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
		if _, err := Decode(s); !errors.Is(err, httpx.ErrInvalidCursor) {
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
	if p := newPage(rows, 3, of); len(p.Items) != 3 || p.Next != "" {
		t.Fatalf("%+v", p)
	}
	p := newPage(rows, 2, of)
	if len(p.Items) != 2 || p.Next == "" {
		t.Fatalf("%+v", p)
	}
	c, err := Decode(p.Next)
	if err != nil || c.ID != "b" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestFetch(t *testing.T) {
	now := time.Now()
	rows := []string{"a", "b", "c"}
	var gotLimit int
	var gotAfter *Cursor
	find := func(_ context.Context, limit int, after *Cursor) ([]string, error) {
		gotLimit, gotAfter = limit, after
		return rows, nil
	}
	of := func(id string) Cursor { return Cursor{CreatedAt: now, ID: id} }

	first, err := Fetch(t.Context(), Params{Limit: 2}, find, of)
	if err != nil || len(first.Items) != 2 || first.Next == "" || gotLimit != 2 || gotAfter != nil {
		t.Fatalf("%+v %v limit %d after %v", first, err, gotLimit, gotAfter)
	}
	if _, err := Fetch(t.Context(), Params{Limit: 2, Cursor: first.Next}, find, of); err != nil || gotAfter == nil || gotAfter.ID != "b" {
		t.Fatalf("after %v err %v", gotAfter, err)
	}
}

func TestFetchErrors(t *testing.T) {
	of := func(id string) Cursor { return Cursor{ID: id} }
	never := func(context.Context, int, *Cursor) ([]string, error) {
		t.Fatal("find must not be called with a bad cursor")
		return nil, nil
	}
	if _, err := Fetch(t.Context(), Params{Limit: 1, Cursor: "!!!"}, never, of); !errors.Is(err, httpx.ErrInvalidCursor) {
		t.Fatalf("err %v", err)
	}
	boom := errors.New("boom")
	failing := func(context.Context, int, *Cursor) ([]string, error) { return nil, boom }
	if _, err := Fetch(t.Context(), Params{Limit: 1}, failing, of); !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
}

func TestMapKeepsCursorAndNeverReturnsNilItems(t *testing.T) {
	got := Map(Page[int]{Next: "n"}, func(i int) string { return "x" })
	if got.Items == nil || len(got.Items) != 0 || got.Next != "n" {
		t.Fatalf("%+v", got)
	}
	got = Map(Page[int]{Items: []int{1, 2}}, func(i int) string { return string(rune('a' + i)) })
	if len(got.Items) != 2 || got.Items[0] != "b" || got.Items[1] != "c" {
		t.Fatalf("%+v", got)
	}
}
