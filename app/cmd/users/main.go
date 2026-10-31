package main

import (
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/users"
)

func main() {
	app.Run("users", users.Setup)
}
