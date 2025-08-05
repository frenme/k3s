package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

type ServerConfig struct {
	Addr         string
	DrainDelay   time.Duration
	GraceTimeout time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func DefaultServerConfig(addr string) ServerConfig {
	return ServerConfig{
		Addr:         addr,
		DrainDelay:   5 * time.Second,
		GraceTimeout: 20 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

