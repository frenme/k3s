package main

import (
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/orders"
)

func main() {
	app.Run("orders", orders.Setup)
}
