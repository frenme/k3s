package app

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/egor/k3s-base/internal/lib/cache"
	"github.com/egor/k3s-base/internal/lib/envconfig"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/lock"
	"github.com/egor/k3s-base/internal/lib/pg"
	"github.com/redis/go-redis/v9"
)

type EnvConfig struct {
	LogLevel      string `env:"LOG_LEVEL" default:"info"`
	RedisAddr     string `env:"REDIS_ADDR"`
	RedisPassword string `env:"REDIS_PASSWORD"`
}

type Service struct {
	Name     string
	Public   bool
	Env      EnvConfig
	Log      *slog.Logger
	Health   *httpx.Health
	Mux      *http.ServeMux
	Internal *http.ServeMux
	closers  []func()
	redis    *redis.Client
}

func Run(name string, setup func(ctx context.Context, s *Service) error) {
	var env EnvConfig
	envErr := envconfig.Load(&env)
	s := &Service{
		Name:     name,
		Env:      env,
		Log:      newLogger(name, env.LogLevel),
		Health:   httpx.NewHealth(),
		Mux:      http.NewServeMux(),
		Internal: http.NewServeMux(),
	}
	if envErr != nil {
		s.Log.Error("fatal", "err", envErr)
		os.Exit(1)
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
	shutdown, err := setupTracing(ctx, s.Name)
	if err != nil {
		return err
	}
	s.OnClose(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	})
	if err := setup(ctx, s); err != nil {
		return err
	}
	return httpx.Serve(ctx, s.Name, s.Public, s.Log, s.Health, s.Mux, s.Internal)
}

func (s *Service) OnClose(fn func()) {
	s.closers = append(s.closers, fn)
}

func (s *Service) close() {
	for i := len(s.closers) - 1; i >= 0; i-- {
		s.closers[i]()
	}
}

func (s *Service) Postgres(ctx context.Context, fsys fs.FS, dir string) (*pg.DB, error) {
	pools, err := pg.Open(ctx, s.Name, fsys, dir)
	if err != nil {
		return nil, err
	}
	s.OnClose(pools.Close)
	s.Health.Add("postgres", pools.Write.Ping)
	return pg.NewDB(pools)
}

func (s *Service) Redis() *redis.Client {
	if s.redis == nil && s.Env.RedisAddr != "" {
		s.redis = cache.NewClient(s.Env.RedisAddr, s.Env.RedisPassword)
		client := s.redis
		s.OnClose(func() { _ = client.Close() })
	}
	return s.redis
}

func (s *Service) Cache() *cache.Cache {
	return cache.New(s.Redis(), s.Name)
}

func (s *Service) Locker() *lock.Locker {
	owner, _ := os.Hostname()
	return lock.New(s.Redis(), s.Name, owner)
}
