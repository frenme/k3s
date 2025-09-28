package users

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	items map[string]User
}

func (f *fakeStore) List(ctx context.Context, limit int) ([]User, error) {
	out := []User{}
	for _, u := range f.items {
		out = append(out, u)
	}
	return out, nil
}

func (f *fakeStore) Get(ctx context.Context, id string) (User, error) {
	u, ok := f.items[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) Create(ctx context.Context, email, name string) (User, error) {
	for _, u := range f.items {
		if u.Email == email {
			return User{}, ErrDuplicate
		}
	}
	u := User{ID: "id-" + email, Email: email, Name: name, CreatedAt: time.Now()}
	f.items[u.ID] = u
	return u, nil
}

