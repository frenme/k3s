package v1

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egor/k3s-base/internal/common/config"
	"github.com/egor/k3s-base/internal/common/model"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type fakeStore map[string]model.Config

func (f fakeStore) ListPublic(ctx context.Context) ([]model.Config, error) {
	var out []model.Config
	for _, s := range f {
		if s.Public {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f fakeStore) Get(ctx context.Context, key string) (model.Config, error) {
	s, ok := f[key]
	if !ok {
		return model.Config{}, model.ErrConfigNotFound
	}
	return s, nil
}

func get(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func TestConfig(t *testing.T) {
	store := fakeStore{
		"registration_enabled": {Key: "registration_enabled", Value: json.RawMessage(`true`), Public: true},
		"internal_limit":       {Key: "internal_limit", Value: json.RawMessage(`{"max":5}`)},
	}
	pubMux, inMux := http.NewServeMux(), http.NewServeMux()
	NewHandler(config.NewService(store), slog.New(slog.NewTextHandler(io.Discard, nil))).Register(httpx.NewAPI(pubMux, "common"), httpx.NewInternalAPI(inMux, "common"))
	pub, in := httptest.NewServer(pubMux), httptest.NewServer(inMux)
	defer pub.Close()
	defer in.Close()

	status, body := get(t, pub.URL+"/v1/common/config")
	items, _ := body["items"].([]any)
	if status != http.StatusOK || len(items) != 1 {
		t.Fatalf("public %d %v", status, body)
	}
	if status, body := get(t, in.URL+"/internal/v1/common/config/internal_limit"); status != http.StatusOK || body["key"] != "internal_limit" {
		t.Fatalf("internal %d %v", status, body)
	}
	if status, body := get(t, in.URL+"/internal/v1/common/config/nope"); status != http.StatusNotFound || body["code"] != "config_not_found" {
		t.Fatalf("missing %d %v", status, body)
	}
	if status, _ := get(t, pub.URL+"/internal/v1/common/config/internal_limit"); status != http.StatusNotFound {
		t.Fatalf("internal route on public api: %d", status)
	}
}
