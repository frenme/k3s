package main

import (
	"github.com/egor/k3s-base/internal/common"
	"github.com/egor/k3s-base/internal/lib/app"
)

func main() {
	app.Run("common", common.Setup)
}
