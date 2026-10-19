package app

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/egor/k3s-base/internal/lib/cache"
	"github.com/egor/k3s-base/internal/lib/config"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/logx"
	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/egor/k3s-base/internal/lib/rds"
)

type Service struct {
	Name    string
	Log     *slog.Logger
	Health  *httpx.Health
	Mux     *http.ServeMux
	closers []func()
}

func Run(name string, setup func(ctx context.Context, s *Service) error) {
	s := &Service{
		Name:   name,
		Log:    logx.New(name, config.String("LOG_LEVEL", "info")),
		Health: httpx.NewHealth(),
		Mux:    http.NewServeMux(),
	}
	s.Health.Mount(s.Mux)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := s.run(ctx, setup)
	stop()
	if err != nil {
		s.Log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func (s *Service) run(ctx context.Context, setup func(ctx context.Context, s *Service) error) error {
	defer s.close()
	if err := setup(ctx, s); err != nil {
		return err
	}
	return httpx.Serve(ctx, s.Log, s.Health, s.Mux)
}

func (s *Service) OnClose(fn func()) {
	s.closers = append(s.closers, fn)
}

func (s *Service) close() {
	for i := len(s.closers) - 1; i >= 0; i-- {
		s.closers[i]()
	}
}

func (s *Service) Postgres(ctx context.Context, fsys fs.FS, dir string) (*pg.Pools, error) {
	pools, err := pg.Open(ctx, s.Name, fsys, dir)
	if err != nil {
		return nil, err
	}
	s.OnClose(pools.Close)
	s.Health.Add("postgres", pools.Write.Ping)
	return pools, nil
}

func (s *Service) Cache() *cache.Cache {
	addr := config.String("REDIS_ADDR", "")
	if addr == "" {
		return nil
	}
	client := rds.New(addr, config.String("REDIS_PASSWORD", ""))
	s.OnClose(func() { _ = client.Close() })
	return cache.New(client, s.Name)
}
