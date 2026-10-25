package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/egor/k3s-base/internal/lib/config"
)

type ServerConfig struct {
	Addr         string
	InternalAddr string
	DrainDelay   time.Duration
	GraceTimeout time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

func ServerConfigFromEnv() ServerConfig {
	return ServerConfig{
		Addr:         config.String("HTTP_ADDR", ":8080"),
		InternalAddr: config.String("INTERNAL_ADDR", ":8081"),
		DrainDelay:   config.Duration("DRAIN_DELAY", 5*time.Second),
		GraceTimeout: config.Duration("GRACE_TIMEOUT", 20*time.Second),
		ReadTimeout:  config.Duration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout: config.Duration("WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:  config.Duration("IDLE_TIMEOUT", 60*time.Second),
	}
}

type Listener struct {
	Addr    string
	Handler http.Handler
}

func Serve(ctx context.Context, name string, public bool, log *slog.Logger, health *Health, mux, internal *http.ServeMux) error {
	cfg := ServerConfigFromEnv()
	return Run(ctx, cfg, health, log,
		Listener{Addr: cfg.Addr, Handler: Chain(JSONErrors(mux), Trace(name, public), Recover(log), Observe(log))},
		Listener{Addr: cfg.InternalAddr, Handler: Chain(JSONErrors(internal), Trace(name, false), Recover(log), Observe(log))},
	)
}

func Run(ctx context.Context, cfg ServerConfig, health *Health, log *slog.Logger, listeners ...Listener) error {
	servers := make([]*http.Server, 0, len(listeners))
	errCh := make(chan error, len(listeners))
	for _, l := range listeners {
		srv := &http.Server{
			Addr:              l.Addr,
			Handler:           l.Handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		}
		servers = append(servers, srv)
		go func() {
			log.Info("listening", "addr", srv.Addr)
			errCh <- srv.ListenAndServe()
		}()
	}
	select {
	case err := <-errCh:
		shutdown(servers)
		return err
	case <-ctx.Done():
	}
	log.Info("shutdown requested", "drain", cfg.DrainDelay.String())
	health.SetReady(false)
	time.Sleep(cfg.DrainDelay)
	sctx, cancel := context.WithTimeout(context.Background(), cfg.GraceTimeout)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	log.Info("shutdown complete")
	return nil
}

func shutdown(servers []*http.Server) {
	for _, srv := range servers {
		_ = srv.Close()
	}
}
