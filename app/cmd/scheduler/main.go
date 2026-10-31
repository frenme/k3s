package main

import (
	"github.com/egor/k3s-base/internal/lib/app"
	"github.com/egor/k3s-base/internal/scheduler"
)

func main() {
	app.Run("scheduler", scheduler.Setup)
}
