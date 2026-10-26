package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/egor/k3s-base/internal/lib/lock"
	"github.com/egor/k3s-base/internal/scheduler/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/robfig/cron/v3"
)

const (
	TriggerCron   = "cron"
	TriggerManual = "manual"

	lockMargin = 5 * time.Second
)

var (
	runs = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "scheduler_job_runs_total",
		Help: "Job runs by result and trigger.",
	}, []string{"job", "trigger", "result"})
	duration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "scheduler_job_duration_seconds",
		Help:    "Job run time.",
		Buckets: []float64{.1, .5, 1, 5, 15, 60, 300, 900},
	}, []string{"job"})
	lastSuccess = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "scheduler_job_last_success_timestamp_seconds",
		Help: "Unix time of the last successful run.",
	}, []string{"job"})
)

type Job struct {
	Name     string
	Schedule string
	Timeout  time.Duration
	Run      func(ctx context.Context) error
}

type Locker interface {
	Acquire(ctx context.Context, name string, ttl time.Duration) (func(), error)
}

type entry struct {
	job      Job
	schedule cron.Schedule
}

type Service struct {
	log     *slog.Logger
	locker  Locker
	entries map[string]*entry
	cron    *cron.Cron
}

func NewService(log *slog.Logger, locker Locker, jobs []Job) (*Service, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	s := &Service{log: log, locker: locker, entries: map[string]*entry{}, cron: cron.New(cron.WithParser(parser), cron.WithLocation(time.UTC))}
	for _, j := range jobs {
		if _, dup := s.entries[j.Name]; dup {
			return nil, fmt.Errorf("duplicate job %q", j.Name)
		}
		if j.Timeout <= 0 {
			return nil, fmt.Errorf("job %q has no timeout", j.Name)
		}
		schedule, err := parser.Parse(j.Schedule)
		if err != nil {
			return nil, fmt.Errorf("job %q: %w", j.Name, err)
		}
		e := &entry{job: j, schedule: schedule}
		s.entries[j.Name] = e
		s.cron.Schedule(schedule, cron.FuncJob(func() { s.tick(e) }))
	}
	return s, nil
}

func (s *Service) Start() {
	s.cron.Start()
}

func (s *Service) Stop() <-chan struct{} {
	return s.cron.Stop().Done()
}

func (s *Service) List() []model.Job {
	now := time.Now()
	out := make([]model.Job, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, model.Job{Name: e.job.Name, Schedule: e.job.Schedule, Timeout: e.job.Timeout, NextRun: e.schedule.Next(now).UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Service) Run(ctx context.Context, name string) (model.Run, error) {
	e, ok := s.entries[name]
	if !ok {
		return model.Run{}, model.ErrJobNotFound
	}
	return s.execute(context.WithoutCancel(ctx), e, TriggerManual)
}

func (s *Service) tick(e *entry) {
	ctx := context.Background()
	now := time.Now()
	ttl := max(e.schedule.Next(now).Sub(now)-time.Second/2, time.Second)
	if _, err := s.locker.Acquire(ctx, "tick:"+e.job.Name, ttl); err != nil {
		if !errors.Is(err, lock.ErrHeld) {
			s.log.Error("job tick lock failed", "job", e.job.Name, "err", err)
		}
		return
	}
	if _, err := s.execute(ctx, e, TriggerCron); err != nil && !errors.Is(err, model.ErrJobRunning) {
		s.log.Error("job did not start", "job", e.job.Name, "err", err)
	}
}

func (s *Service) execute(ctx context.Context, e *entry, trigger string) (model.Run, error) {
	release, err := s.locker.Acquire(ctx, "run:"+e.job.Name, e.job.Timeout+lockMargin)
	if errors.Is(err, lock.ErrHeld) {
		runs.WithLabelValues(e.job.Name, trigger, "skipped").Inc()
		return model.Run{}, model.ErrJobRunning
	}
	if err != nil {
		return model.Run{}, err
	}
	defer release()

	ctx, cancel := context.WithTimeout(ctx, e.job.Timeout)
	defer cancel()
	start := time.Now()
	runErr := safely(ctx, e.job.Run)
	elapsed := time.Since(start)

	result := "success"
	run := model.Run{Job: e.job.Name, Duration: elapsed}
	if runErr != nil {
		result, run.Error = "error", runErr.Error()
		s.log.Error("job failed", "job", e.job.Name, "trigger", trigger, "duration_ms", elapsed.Milliseconds(), "err", runErr)
	} else {
		lastSuccess.WithLabelValues(e.job.Name).SetToCurrentTime()
		s.log.Info("job finished", "job", e.job.Name, "trigger", trigger, "duration_ms", elapsed.Milliseconds())
	}
	runs.WithLabelValues(e.job.Name, trigger, result).Inc()
	duration.WithLabelValues(e.job.Name).Observe(elapsed.Seconds())
	return run, nil
}

func safely(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(ctx)
}
