package v1

import (
	"encoding/json"

	"github.com/egor/k3s-base/internal/common/model"
)

type Setting struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

func toSetting(s model.Setting) Setting {
	return Setting{Key: s.Key, Value: s.Value}
}

type ListSettingsResponse struct {
	Body struct {
		Items []Setting `json:"items"`
	}
}

type GetSettingRequest struct {
	Key string `path:"key" pattern:"^[a-z0-9_]+$" maxLength:"100"`
}

type SettingResponse struct {
	Body Setting
}
