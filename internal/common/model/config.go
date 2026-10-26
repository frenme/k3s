package model

import (
	"encoding/json"
	"time"
)

type Config struct {
	Key       string
	Value     json.RawMessage
	Public    bool
	UpdatedAt time.Time
}
