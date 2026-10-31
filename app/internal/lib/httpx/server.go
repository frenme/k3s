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

type Server struct {
	Name     string
	Public   bool
	Log      *slog.Logger
	Probes   *Probes
	Mux      *http.ServeMux
	Internal *http.ServeMux
}

func Serve(ctx context.Context, s Server) error {
	cfg, err := envconfig.Load[ServerConfig]()
	if err != nil {
		return err
	}
	servers := []*http.Server{
		newServer(cfg, cfg.Addr, Chain(JSONErrors(s.Mux), Trace(s.Name, s.Public), Observe(s.Log), Recover(s.Log))),
		newServer(cfg, cfg.InternalAddr, Chain(JSONErrors(s.Internal), Trace(s.Name, false), Observe(s.Log), Recover(s.Log))),
	}
	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		go func() {
			s.Log.Info("listening", "addr", srv.Addr)
			errCh <- srv.ListenAndServe()
		}()
	}
	select {
	case err := <-errCh:
		for _, srv := range servers {
			_ = srv.Close()
		}
		return err
	case <-ctx.Done():
	}
	s.Log.Info("shutdown requested", "drain", cfg.DrainDelay.String())
	s.Probes.SetReady(false)
	time.Sleep(cfg.DrainDelay)
	sctx, cancel := context.WithTimeout(context.Background(), cfg.GraceTimeout)
	defer cancel()
	for _, srv := range servers {
		if err := srv.Shutdown(sctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	s.Log.Info("shutdown complete")
	return nil
}

func newServer(cfg ServerConfig, addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}
