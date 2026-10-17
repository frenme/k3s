package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/egor/k3s-base/internal/platform/config"
)

type ServerConfig struct {
	Addr         string
	DrainDelay   time.Duration
	GraceTimeout time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func ServerConfigFromEnv() ServerConfig {
	return ServerConfig{
		Addr:         config.String("HTTP_ADDR", ":8080"),
		DrainDelay:   config.Duration("DRAIN_DELAY", 5*time.Second),
		GraceTimeout: config.Duration("GRACE_TIMEOUT", 20*time.Second),
		ReadTimeout:  config.Duration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout: config.Duration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:  config.Duration("IDLE_TIMEOUT", 60*time.Second),
	}
}

func Serve(ctx context.Context, log *slog.Logger, health *Health, mux http.Handler) error {
	handler := Chain(mux, RequestID(), Recover(log), Observe(log))
	return Run(ctx, ServerConfigFromEnv(), handler, health, log)
}

func Run(ctx context.Context, cfg ServerConfig, handler http.Handler, health *Health, log *slog.Logger) error {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	log.Info("shutdown requested", "drain", cfg.DrainDelay.String())
	health.SetReady(false)
	time.Sleep(cfg.DrainDelay)
	sctx, cancel := context.WithTimeout(context.Background(), cfg.GraceTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("shutdown complete")
	return nil
}
