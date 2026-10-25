package main

import (
	"github.com/egor/k3s-base/internal/admin"
	"github.com/egor/k3s-base/internal/lib/app"
)

func main() {
	app.Run("admin", admin.Setup)
}
