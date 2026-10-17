package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/egor/k3s-base/internal/platform/rds"
)

type item struct {
	Name string `json:"name"`
}

func TestFetchLoadsOnceThenHits(t *testing.T) {
	mr := miniredis.RunT(t)
	c := New(rds.New(mr.Addr(), ""), "test")
	calls := 0
	load := func() (item, error) {
		calls++
		return item{Name: "a"}, nil
	}
	for i := 0; i < 3; i++ {
		v, err := Fetch(context.Background(), c, "k", time.Minute, load)
		if err != nil || v.Name != "a" {
			t.Fatalf("v %v err %v", v, err)
		}
	}
	if calls != 1 {
		t.Fatalf("loader called %d times", calls)
	}
}

func TestFetchDoesNotCacheErrors(t *testing.T) {
	mr := miniredis.RunT(t)
	c := New(rds.New(mr.Addr(), ""), "test")
	boom := errors.New("boom")
	calls := 0
	load := func() (item, error) {
		calls++
		return item{}, boom
	}
	for i := 0; i < 2; i++ {
		if _, err := Fetch(context.Background(), c, "k", time.Minute, load); !errors.Is(err, boom) {
			t.Fatalf("err %v", err)
		}
	}
	if calls != 2 {
		t.Fatalf("loader called %d times", calls)
	}
}

func TestFetchExpires(t *testing.T) {
	mr := miniredis.RunT(t)
	c := New(rds.New(mr.Addr(), ""), "test")
	calls := 0
	load := func() (item, error) {
		calls++
		return item{Name: "a"}, nil
	}
	_, _ = Fetch(context.Background(), c, "k", time.Second, load)
	mr.FastForward(2 * time.Second)
	_, _ = Fetch(context.Background(), c, "k", time.Second, load)
	if calls != 2 {
		t.Fatalf("loader called %d times", calls)
	}
}

func TestDeleteForcesReload(t *testing.T) {
	mr := miniredis.RunT(t)
	c := New(rds.New(mr.Addr(), ""), "test")
	calls := 0
	load := func() (item, error) {
		calls++
		return item{Name: "a"}, nil
	}
	_, _ = Fetch(context.Background(), c, "k", time.Minute, load)
	c.Delete(context.Background(), "k")
	_, _ = Fetch(context.Background(), c, "k", time.Minute, load)
	if calls != 2 {
		t.Fatalf("loader called %d times", calls)
	}
}

func TestFetchFallsBackWhenRedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	c := New(rds.New(mr.Addr(), ""), "test")
	mr.Close()
	v, err := Fetch(context.Background(), c, "k", time.Minute, func() (item, error) { return item{Name: "x"}, nil })
	if err != nil || v.Name != "x" {
		t.Fatalf("v %v err %v", v, err)
	}
}

func TestNilCacheLoadsDirectly(t *testing.T) {
	var c *Cache
	v, err := Fetch(context.Background(), c, "k", time.Minute, func() (item, error) { return item{Name: "n"}, nil })
	if err != nil || v.Name != "n" {
		t.Fatalf("v %v err %v", v, err)
	}
	c.Delete(context.Background(), "k")
}
