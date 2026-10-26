package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/egor/k3s-base/internal/lib/envconfig"
)

type ServerConfig struct {
	Addr         string        `env:"HTTP_ADDR" default:":8080"`
	InternalAddr string        `env:"INTERNAL_ADDR" default:":8081"`
	DrainDelay   time.Duration `env:"DRAIN_DELAY" default:"5s"`
	GraceTimeout time.Duration `env:"GRACE_TIMEOUT" default:"20s"`
	ReadTimeout  time.Duration `env:"READ_TIMEOUT" default:"10s"`
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" default:"30s"`
	IdleTimeout  time.Duration `env:"IDLE_TIMEOUT" default:"60s"`
}

type Listener struct {
	Addr    string
	Handler http.Handler
}

func Serve(ctx context.Context, name string, public bool, log *slog.Logger, health *Health, mux, internal *http.ServeMux) error {
	var cfg ServerConfig
	if err := envconfig.Load(&cfg); err != nil {
		return err
	}
	return Run(ctx, cfg, health, log,
		Listener{Addr: cfg.Addr, Handler: Chain(JSONErrors(mux), Trace(name, public), Observe(log), Recover(log))},
		Listener{Addr: cfg.InternalAddr, Handler: Chain(JSONErrors(internal), Trace(name, false), Observe(log), Recover(log))},
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
