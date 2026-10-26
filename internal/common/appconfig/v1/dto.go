package v1

import (
	"encoding/json"

	"github.com/egor/k3s-base/internal/common/model"
)

type Config struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func toConfig(s model.Config) Config {
	return Config{Key: s.Key, Value: s.Value}
}

type ListConfigResponse struct {
	Body struct {
		Items []Config `json:"items"`
	}
}

type GetConfigRequest struct {
	Key string `path:"key" pattern:"^[a-z0-9_]+$" maxLength:"100"`
}

type ConfigResponse struct {
	Body Config
}
