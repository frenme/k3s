package pg

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type poolCollector struct {
	pool                                      *pgxpool.Pool
	acquired, idle, total, max, waiting       *prometheus.Desc
	acquires, emptyAcquires, canceledAcquires *prometheus.Desc
	acquireSeconds, emptyAcquireWaitSeconds   *prometheus.Desc
}

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.acquired, c.idle, c.total, c.max, c.waiting, c.acquires, c.emptyAcquires, c.canceledAcquires, c.acquireSeconds, c.emptyAcquireWaitSeconds} {
		ch <- d
	}
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	gauge := func(d *prometheus.Desc, v float64) { ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v) }
	counter := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v)
	}
	gauge(c.acquired, float64(s.AcquiredConns()))
	gauge(c.idle, float64(s.IdleConns()))
	gauge(c.total, float64(s.TotalConns()))
	gauge(c.max, float64(s.MaxConns()))
	gauge(c.waiting, float64(s.ConstructingConns()))
	counter(c.acquires, float64(s.AcquireCount()))
	counter(c.emptyAcquires, float64(s.EmptyAcquireCount()))
	counter(c.canceledAcquires, float64(s.CanceledAcquireCount()))
	counter(c.acquireSeconds, s.AcquireDuration().Seconds())
	counter(c.emptyAcquireWaitSeconds, s.EmptyAcquireWaitTime().Seconds())
}

func newPoolCollector(service, role string, pool *pgxpool.Pool) *poolCollector {
	labels := prometheus.Labels{"service": service, "pool": role}
	desc := func(name, help string) *prometheus.Desc {
		return prometheus.NewDesc("db_pool_"+name, help, nil, labels)
	}
	return &poolCollector{
		pool:                    pool,
		acquired:                desc("acquired_connections", "Connections currently in use."),
		idle:                    desc("idle_connections", "Idle connections."),
		total:                   desc("open_connections", "Open connections."),
		max:                     desc("max_connections", "Configured pool size."),
		waiting:                 desc("constructing_connections", "Connections being established."),
		acquires:                desc("acquires_total", "Successful connection acquires."),
		emptyAcquires:           desc("empty_acquires_total", "Acquires that had to wait for a new connection."),
		canceledAcquires:        desc("canceled_acquires_total", "Acquires canceled by the caller."),
		acquireSeconds:          desc("acquire_seconds_total", "Total time spent acquiring connections."),
		emptyAcquireWaitSeconds: desc("empty_acquire_wait_seconds_total", "Total time spent waiting for a new connection."),
	}
}
