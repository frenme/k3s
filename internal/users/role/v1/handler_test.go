package v1

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/users/model"
	"github.com/egor/k3s-base/internal/users/role"
	"github.com/google/uuid"
)

type fakeStore struct {
	roles map[string][]model.Role
}

func (f *fakeStore) Assigned(_ context.Context, id string) ([]model.Role, error) {
	r, ok := f.roles[id]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	return r, nil
}

func (f *fakeStore) Grant(_ context.Context, id string, r model.Role) error {
	if _, ok := f.roles[id]; !ok {
		return model.ErrUserNotFound
	}
	f.roles[id] = append(f.roles[id], r)
	return nil
}

func (f *fakeStore) Revoke(_ context.Context, id string, r model.Role) error {
	if _, ok := f.roles[id]; !ok {
		return model.ErrUserNotFound
	}
	f.roles[id] = slices.DeleteFunc(f.roles[id], func(x model.Role) bool { return x == r })
	return nil
}

func servers(t *testing.T, id string) (public, internal string) {
	t.Helper()
	svc := role.NewService(&fakeStore{roles: map[string][]model.Role{id: nil}})
	pubMux, inMux := http.NewServeMux(), http.NewServeMux()
	NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(httpx.NewAPI(pubMux, "users"), httpx.NewInternalAPI(inMux, "users"))
	pub, in := httptest.NewServer(pubMux), httptest.NewServer(inMux)
	t.Cleanup(pub.Close)
	t.Cleanup(in.Close)
	return pub.URL, in.URL
}

func do(t *testing.T, method, url string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(method, url, strings.NewReader(""))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestRolesLifecycle(t *testing.T) {
	id := uuid.NewString()
	public, internal := servers(t, id)

	if status, body := do(t, http.MethodGet, internal+"/internal/v1/users/"+id+"/roles"); status != http.StatusOK || len(body["roles"].([]any)) != 1 {
		t.Fatalf("internal: %d %v", status, body)
	}
	if status, body := do(t, http.MethodPut, public+"/v1/users/"+id+"/roles/admin"); status != http.StatusOK || len(body["roles"].([]any)) != 2 {
		t.Fatalf("grant: %d %v", status, body)
	}
	if status, body := do(t, http.MethodDelete, public+"/v1/users/"+id+"/roles/admin"); status != http.StatusOK || len(body["roles"].([]any)) != 1 {
		t.Fatalf("revoke: %d %v", status, body)
	}
}

func TestRoleErrors(t *testing.T) {
	id := uuid.NewString()
	public, internal := servers(t, id)
	if status, body := do(t, http.MethodPut, public+"/v1/users/"+id+"/roles/root"); status != http.StatusUnprocessableEntity || body["code"] != "invalid_input" {
		t.Fatalf("unknown role: %d %v", status, body)
	}
	if status, body := do(t, http.MethodGet, internal+"/internal/v1/users/"+uuid.NewString()+"/roles"); status != http.StatusNotFound || body["code"] != "user_not_found" {
		t.Fatalf("unknown user: %d %v", status, body)
	}
}

func TestInternalRouteStaysOffThePublicAPI(t *testing.T) {
	id := uuid.NewString()
	public, _ := servers(t, id)
	if status, _ := do(t, http.MethodGet, public+"/internal/v1/users/"+id+"/roles"); status != http.StatusNotFound {
		t.Fatalf("status %d", status)
	}
}
