package main

import (
	"context"

	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/config"
	"github.com/egor/k3s-base/internal/orders"
)

func main() {
	app.Run("orders", func(ctx context.Context, s *app.Service) error {
		pools, err := s.Postgres(ctx, orders.Migrations, orders.MigrationsDir)
		if err != nil {
			return err
		}
		usersClient := orders.NewUsersClient(config.String("USERS_URL", "http://users:8080"))
		orders.NewHandler(orders.NewPgStore(pools), usersClient, s.Log).Mount(s.Mux)
		return nil
	})
}
