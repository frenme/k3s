package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/egor/k3s-base/internal/lib/lock"
	"github.com/egor/k3s-base/internal/scheduler/model"
	"github.com/redis/go-redis/v9"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func lockers(t *testing.T, owners ...string) []*lock.Locker {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	out := make([]*lock.Locker, 0, len(owners))
	for _, owner := range owners {
		out = append(out, lock.New(client, "scheduler", owner))
	}
	return out
}

func job(name string, run func(context.Context) error) Job {
	return Job{Name: name, Schedule: "@every 1h", Timeout: time.Minute, Run: run}
}

func TestNewServiceValidates(t *testing.T) {
	l := lockers(t, "a")[0]
	noop := func(context.Context) error { return nil }
	cases := map[string][]Job{
		"duplicate":    {job("x", noop), job("x", noop)},
		"bad schedule": {{Name: "x", Schedule: "nope", Timeout: time.Second, Run: noop}},
		"no timeout":   {{Name: "x", Schedule: "@every 1h", Run: noop}},
	}
	for name, jobs := range cases {
		if _, err := NewService(discard(), l, jobs); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestRunManualReportsResult(t *testing.T) {
	l := lockers(t, "a")[0]
	svc, err := NewService(discard(), l, []Job{
		job("ok", func(context.Context) error { return nil }),
		job("bad", func(context.Context) error { return errors.New("boom") }),
		job("panics", func(context.Context) error { panic("kaboom") }),
	})
	if err != nil {
		t.Fatal(err)
	}
	if run, err := svc.Run(t.Context(), "ok"); err != nil || !run.Succeeded() {
		t.Fatalf("ok: %+v %v", run, err)
	}
	if run, err := svc.Run(t.Context(), "bad"); err != nil || run.Succeeded() || run.Error != "boom" {
		t.Fatalf("bad: %+v %v", run, err)
	}
	if run, err := svc.Run(t.Context(), "panics"); err != nil || run.Succeeded() {
		t.Fatalf("panics: %+v %v", run, err)
	}
	if _, err := svc.Run(t.Context(), "missing"); !errors.Is(err, model.ErrJobNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestRunRefusesOverlap(t *testing.T) {
	a := lockers(t, "pod-a")[0]
	started, finish := make(chan struct{}), make(chan struct{})
	svc, _ := NewService(discard(), a, []Job{job("slow", func(context.Context) error {
		close(started)
		<-finish
		return nil
	})})
	done := make(chan error, 1)
	go func() {
		_, err := svc.Run(context.Background(), "slow")
		done <- err
	}()
	<-started
	if _, err := svc.Run(t.Context(), "slow"); !errors.Is(err, model.ErrJobRunning) {
		t.Fatalf("overlap: %v", err)
	}
	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Run(t.Context(), "slow"); err != nil && errors.Is(err, model.ErrJobRunning) {
		t.Fatalf("lock was not released: %v", err)
	}
}

func TestReplicasRunEachTickOnce(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	var runs atomic.Int32
	counted := job("tick", func(context.Context) error {
		runs.Add(1)
		return nil
	})
	var services []*Service
	for _, owner := range []string{"pod-a", "pod-b", "pod-c"} {
		svc, err := NewService(discard(), lock.New(client, "scheduler", owner), []Job{counted})
		if err != nil {
			t.Fatal(err)
		}
		services = append(services, svc)
	}

	var wg sync.WaitGroup
	for _, svc := range services {
		wg.Go(func() { svc.tick(svc.entries["tick"]) })
	}
	wg.Wait()
	if got := runs.Load(); got != 1 {
		t.Fatalf("job ran %d times for one tick", got)
	}

	mr.FastForward(2 * time.Hour)
	services[1].tick(services[1].entries["tick"])
	if got := runs.Load(); got != 2 {
		t.Fatalf("next tick did not run: %d", got)
	}
}

func TestListIsSortedWithNextRun(t *testing.T) {
	l := lockers(t, "a")[0]
	noop := func(context.Context) error { return nil }
	svc, _ := NewService(discard(), l, []Job{job("b", noop), job("a", noop)})
	items := svc.List()
	if len(items) != 2 || items[0].Name != "a" || !items[0].NextRun.After(time.Now()) {
		t.Fatalf("%+v", items)
	}
}
