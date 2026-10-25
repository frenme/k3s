package orders

import (
	"context"

	"github.com/egor/k3s-base/internal/clients/users"
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/lib/config"
	"github.com/egor/k3s-base/internal/orders/checkout"
	"github.com/egor/k3s-base/internal/orders/storage"
)

func Setup(ctx context.Context, s *app.Service) error {
	pools, err := s.Postgres(ctx, storage.Migrations, storage.MigrationsDir)
	if err != nil {
		return err
	}
	usersClient := users.New(config.String("USERS_URL", "http://users:8081"))
	checkout.NewHandler(checkout.NewService(storage.NewOrders(pools), usersClient), s.Log).Mount(s.Mux)
	return nil
}
