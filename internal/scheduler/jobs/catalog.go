package jobs

import (
	"context"
	"log/slog"
	"time"
)

func Catalog(log *slog.Logger) []Job {
	return []Job{
		{
			Name:     "heartbeat",
			Schedule: "@every 1m",
			Timeout:  10 * time.Second,
			Run: func(ctx context.Context) error {
				log.InfoContext(ctx, "scheduler heartbeat")
				return nil
			},
		},
	}
}
