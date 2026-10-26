package v1

import (
	"encoding/json"

	"github.com/egor/k3s-base/internal/common/model"
)

type AppConfig struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func toAppConfig(s model.AppConfig) AppConfig {
	return AppConfig{Key: s.Key, Value: s.Value}
}

type ListAppConfigResponse struct {
	Body struct {
		Items []AppConfig `json:"items"`
	}
}

type GetAppConfigRequest struct {
	Key string `path:"key" pattern:"^[a-z0-9_]+$" maxLength:"100"`
}

type AppConfigResponse struct {
	Body AppConfig
}
