package main

import (
	"context"

	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/users"
)

func main() {
	app.Run("users", func(ctx context.Context, s *app.Service) error {
		pools, err := s.Postgres(ctx, users.Migrations, users.MigrationsDir)
		if err != nil {
			return err
		}
		users.NewHandler(users.NewPgStore(pools, s.Cache()), s.Log).Mount(s.Mux)
		return nil
	})
}
