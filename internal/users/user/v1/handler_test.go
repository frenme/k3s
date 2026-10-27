package v1

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/users/model"
	"github.com/egor/k3s-base/internal/users/user"
	"github.com/google/uuid"
)

type fakeStore struct {
	items map[string]model.User
}

func (f *fakeStore) List(ctx context.Context, limit int, after *pagination.Cursor) ([]model.User, error) {
	all := []model.User{}
	for _, u := range f.items {
		all = append(all, u)
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].CreatedAt.Equal(all[j].CreatedAt) {
			return all[i].CreatedAt.After(all[j].CreatedAt)
		}
		return all[i].ID > all[j].ID
	})
	out := []model.User{}
	for _, u := range all {
		if after != nil && (u.CreatedAt.After(after.CreatedAt) || (u.CreatedAt.Equal(after.CreatedAt) && u.ID >= after.ID)) {
			continue
		}
		out = append(out, u)
		if len(out) == limit+1 {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) Get(ctx context.Context, id string) (model.User, error) {
	u, ok := f.items[id]
	if !ok {
		return model.User{}, model.ErrUserNotFound
	}
	return u, nil
}

func (f *fakeStore) Create(ctx context.Context, email, name string) (model.User, error) {
	for _, u := range f.items {
		if u.Email == email {
			return model.User{}, model.ErrEmailTaken
		}
	}
	u := model.User{ID: uuid.NewString(), Email: email, Name: name, CreatedAt: time.Now()}
	f.items[u.ID] = u
	return u, nil
}

type fakeAppConfig struct {
	registration bool
	down         bool
}

func (f *fakeAppConfig) Bool(ctx context.Context, key string) (bool, error) {
	if f.down {
		return false, httpx.ErrUnavailable
	}
	return f.registration, nil
}

func newServers(t *testing.T) (public, internal string) {
	t.Helper()
	return newServersWith(t, &fakeAppConfig{registration: true})
}

func newServersWith(t *testing.T, cfg *fakeAppConfig) (public, internal string) {
	t.Helper()
	pubMux, inMux := http.NewServeMux(), http.NewServeMux()
	svc := user.NewService(&fakeStore{items: map[string]model.User{}}, cfg)
	NewHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(httpx.NewAPI(pubMux, "users"), httpx.NewInternalAPI(inMux, "users"))
	pub, in := httptest.NewServer(pubMux), httptest.NewServer(inMux)
	t.Cleanup(pub.Close)
	t.Cleanup(in.Close)
	return pub.URL, in.URL
}

func post(t *testing.T, url, body string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func status(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

func TestCreateAndGet(t *testing.T) {
	public, internal := newServers(t)

	code, body := post(t, public+"/v1/users", `{"email":"A@Example.com","name":"Ann"}`)
	if code != http.StatusCreated || body["email"] != "a@example.com" {
		t.Fatalf("create %d %v", code, body)
	}
	id, _ := body["id"].(string)
	if got := status(t, public+"/v1/users/"+id); got != http.StatusOK {
		t.Fatalf("public get %d", got)
	}
	if got := status(t, internal+"/internal/v1/users/"+id); got != http.StatusOK {
		t.Fatalf("internal get %d", got)
	}
}

func TestInternalRoutesStayOffThePublicAPI(t *testing.T) {
	public, internal := newServers(t)
	if got := status(t, public+"/internal/v1/users/"+uuid.NewString()); got != http.StatusNotFound {
		t.Fatalf("public api serves internal route: %d", got)
	}
	if got := status(t, internal+"/v1/users"); got != http.StatusNotFound {
		t.Fatalf("internal api serves public route: %d", got)
	}
}

func TestErrorsAreJSON(t *testing.T) {
	public, _ := newServers(t)
	post(t, public+"/v1/users", `{"email":"a@b.co","name":"x"}`)

	cases := map[string]struct {
		status int
		code   string
	}{
		`{"email":"nope","name":"x"}`:    {http.StatusUnprocessableEntity, "invalid_input"},
		`{"email":"c@b.co","name":""}`:   {http.StatusUnprocessableEntity, "invalid_input"},
		`{"email":"d@b.co","name":"  "}`: {http.StatusUnprocessableEntity, "invalid_input"},
		`{"email":"a@b.co","name":"y"}`:  {http.StatusConflict, "email_taken"},
		`{bad`:                           {http.StatusBadRequest, "bad_request"},
	}
	for in, want := range cases {
		status, body := post(t, public+"/v1/users", in)
		if status != want.status || body["code"] != want.code || body["message"] == "" {
			t.Fatalf("%s: status %d body %v", in, status, body)
		}
	}
	for _, id := range []string{"missing", uuid.NewString()} {
		if got := status(t, public+"/v1/users/"+id); got != http.StatusNotFound {
			t.Fatalf("%s: status %d", id, got)
		}
	}
	if got := status(t, public+"/v1/users?limit=0"); got != http.StatusUnprocessableEntity {
		t.Fatalf("limit=0: status %d", got)
	}
}

func TestRegistrationAppConfig(t *testing.T) {
	cases := []struct {
		cfg    *fakeAppConfig
		status int
		code   string
	}{
		{&fakeAppConfig{registration: false}, http.StatusForbidden, "registration_closed"},
		{&fakeAppConfig{down: true}, http.StatusBadGateway, "upstream_unavailable"},
	}
	for _, c := range cases {
		public, _ := newServersWith(t, c.cfg)
		status, body := post(t, public+"/v1/users", `{"email":"a@b.co","name":"Ann"}`)
		if status != c.status || body["code"] != c.code {
			t.Fatalf("%+v: %d %v", c.cfg, status, body)
		}
	}
}

func TestDocs(t *testing.T) {
	public, _ := newServers(t)
	for _, path := range []string{"/v1/users/openapi.json", "/v1/users/docs"} {
		if got := status(t, public+path); got != http.StatusOK {
			t.Fatalf("%s: status %d", path, got)
		}
	}
}

func TestListPagination(t *testing.T) {
	public, _ := newServers(t)
	for _, email := range []string{"a@b.co", "b@b.co", "c@b.co"} {
		if code, body := post(t, public+"/v1/users", `{"email":"`+email+`","name":"x"}`); code != http.StatusCreated {
			t.Fatalf("create %d %v", code, body)
		}
	}
	fetch := func(query string) (int, []any, string) {
		resp, err := http.Get(public + "/v1/users" + query)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body struct {
			Items      []any  `json:"items"`
			NextCursor string `json:"next_cursor"`
			Code       string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body.Items, body.NextCursor + body.Code
	}
	code, items, next := fetch("?limit=2")
	if code != http.StatusOK || len(items) != 2 || next == "" {
		t.Fatalf("first page %d %v %q", code, items, next)
	}
	code, items, next = fetch("?limit=2&cursor=" + next)
	if code != http.StatusOK || len(items) != 1 || next != "" {
		t.Fatalf("second page %d %v %q", code, items, next)
	}
	if code, _, body := fetch("?cursor=garbage"); code != http.StatusUnprocessableEntity || body != "invalid_cursor" {
		t.Fatalf("bad cursor %d %q", code, body)
	}
}
