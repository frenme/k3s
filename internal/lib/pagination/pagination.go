package pagination

import (
	"context"
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

type Params struct {
	Limit  int    `query:"limit" default:"50" minimum:"1" maximum:"500"`
	Cursor string `query:"cursor" maxLength:"200"`
}

type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"next_cursor,omitempty"`
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

func Fetch[T any](ctx context.Context, p Params, find func(ctx context.Context, limit int, after *Cursor) ([]T, error), cursorOf func(T) Cursor) (Page[T], error) {
	after, err := Decode(p.Cursor)
	if err != nil {
		return Page[T]{}, err
	}
	fetched, err := find(ctx, p.Limit, after)
	if err != nil {
		return Page[T]{}, err
	}
	return newPage(fetched, p.Limit, cursorOf), nil
}

func Map[T, D any](p Page[T], convert func(T) D) Page[D] {
	items := make([]D, 0, len(p.Items))
	for _, item := range p.Items {
		items = append(items, convert(item))
	}
	return Page[D]{Items: items, Next: p.Next}
}

func newPage[T any](fetched []T, limit int, cursorOf func(T) Cursor) Page[T] {
	if len(fetched) <= limit {
		return Page[T]{Items: fetched}
	}
	items := fetched[:limit]
	return Page[T]{Items: items, Next: cursorOf(items[limit-1]).Encode()}
}
