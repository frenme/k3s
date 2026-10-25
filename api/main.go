package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/admin"
	"github.com/egor/k3s-base/internal/common"
	"github.com/egor/k3s-base/internal/orders"
	"github.com/egor/k3s-base/internal/users"
)

func main() {
	usersPublic, usersInternal := users.OpenAPI()
	commonPublic, commonInternal := common.OpenAPI()
	specs := map[string]*huma.OpenAPI{
		"users.yaml":           usersPublic,
		"users.internal.yaml":  usersInternal,
		"orders.yaml":          orders.OpenAPI(),
		"admin.yaml":           admin.OpenAPI(),
		"common.yaml":          commonPublic,
		"common.internal.yaml": commonInternal,
	}
	for file, spec := range specs {
		out, err := spec.DowngradeYAML()
		if err != nil {
			log.Fatalf("%s: %v", file, err)
		}
		if err := os.WriteFile(filepath.Join("api", file), out, 0o600); err != nil {
			log.Fatal(err)
		}
	}
}
