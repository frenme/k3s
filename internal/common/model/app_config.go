package model

import (
	"encoding/json"
	"time"
)

type AppConfig struct {
	Key       string
	Value     json.RawMessage
	Public    bool
	UpdatedAt time.Time
}
