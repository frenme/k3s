package model

import (
	"encoding/json"
	"time"
)

type Setting struct {
	Key       string
	Value     json.RawMessage
	Public    bool
	UpdatedAt time.Time
}
