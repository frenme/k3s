package users

import (
	"context"

	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/users/profile"
	"github.com/egor/k3s-base/internal/users/storage"
)

func Setup(ctx context.Context, s *app.Service) error {
	pools, err := s.Postgres(ctx, storage.Migrations, storage.MigrationsDir)
	if err != nil {
		return err
	}
	users := storage.NewUsers(pools, s.Cache())
	profile.NewHandler(profile.NewService(users), s.Log).Mount(s.Mux, s.Internal)
	return nil
}
