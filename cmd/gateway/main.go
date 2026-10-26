package main

import (
	"github.com/egor/k3s-base/internal/gateway"
	"github.com/egor/k3s-base/internal/lib/app"
)

func main() {
	app.Run("gateway", gateway.Setup)
}
