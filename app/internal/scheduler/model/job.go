package model

import "time"

type Job struct {
	Name     string
	Schedule string
	Timeout  time.Duration
	NextRun  time.Time
}

type Run struct {
	Job      string
	Duration time.Duration
	Error    string
}

func (r Run) Succeeded() bool {
	return r.Error == ""
}
