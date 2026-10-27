package pagination

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Cursor struct {
	CreatedAt time.Time
	ID        string
}

func (c Cursor) Encode() string {
	raw := strconv.FormatInt(c.CreatedAt.UnixMicro(), 10) + "|" + c.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func Decode(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, httpx.ErrInvalidCursor
	}
	micros, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" {
		return nil, httpx.ErrInvalidCursor
	}
	n, err := strconv.ParseInt(micros, 10, 64)
	if err != nil {
		return nil, httpx.ErrInvalidCursor
	}
	return &Cursor{CreatedAt: time.UnixMicro(n).UTC(), ID: id}, nil
}

type Page[T any] struct {
	Items []T
	Next  string
}

func NewPage[T any](fetched []T, limit int, cursorOf func(T) Cursor) Page[T] {
	if len(fetched) <= limit {
		return Page[T]{Items: fetched}
	}
	items := fetched[:limit]
	return Page[T]{Items: items, Next: cursorOf(items[limit-1]).Encode()}
}
