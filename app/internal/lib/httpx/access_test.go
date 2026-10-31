package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

func TestRegisterRecordsAccess(t *testing.T) {
	api := NewAPI(http.NewServeMux(), "demo")
	noop := func(context.Context, *struct{}) (*struct{}, error) { return &struct{}{}, nil }
	for path, access := range map[string]Access{"/v1/demo/public": AccessPublic, "/v1/demo/user": AccessUser, "/v1/demo/admin": AccessAdmin} {
		Register(api, huma.Operation{OperationID: "op" + string(access), Method: http.MethodGet, Path: path}, access, noop)
	}
	paths := api.OpenAPI().Paths
	for path, want := range map[string]Access{"/v1/demo/public": AccessPublic, "/v1/demo/user": AccessUser, "/v1/demo/admin": AccessAdmin} {
		op := paths[path].Get
		if got := op.Extensions[AccessExtension]; got != string(want) {
			t.Errorf("%s: x-access %v, want %s", path, got, want)
		}
		wantSecurity := map[Access]int{AccessPublic: 0, AccessUser: 1, AccessAdmin: 1}[want]
		if len(op.Security) != wantSecurity {
			t.Errorf("%s: security %v", path, op.Security)
		}
	}
	if _, ok := api.OpenAPI().Components.SecuritySchemes["bearer"]; !ok {
		t.Fatal("bearer scheme is not declared")
	}
}

func TestRegisterTurnsHandlerErrorsIntoResponses(t *testing.T) {
	mux := http.NewServeMux()
	public, internal := NewAPI(mux, "demo"), NewInternalAPI(mux, "demo")
	failWith := func(err error) func(context.Context, *struct{}) (*struct{}, error) {
		return func(context.Context, *struct{}) (*struct{}, error) { return nil, err }
	}
	missing := fmt.Errorf("find thing 7: %w", errThingNotFound)
	broken := errors.New("pq: password authentication failed")
	Register(public, huma.Operation{OperationID: "missing", Method: http.MethodGet, Path: "/v1/demo/missing"}, AccessPublic, failWith(missing))
	Register(public, huma.Operation{OperationID: "broken", Method: http.MethodGet, Path: "/v1/demo/broken"}, AccessPublic, failWith(broken))
	RegisterInternal(internal, huma.Operation{OperationID: "internal-broken", Method: http.MethodGet, Path: "/internal/v1/demo/broken"}, failWith(broken))

	var buf bytes.Buffer
	h := Chain(mux, Observe(slog.New(slog.NewJSONHandler(&buf, nil))))
	cases := []struct {
		path   string
		status int
		code   string
		level  string
	}{
		{"/v1/demo/missing", http.StatusNotFound, "thing_not_found", "INFO"},
		{"/v1/demo/broken", http.StatusInternalServerError, "internal_error", "ERROR"},
		{"/internal/v1/demo/broken", http.StatusInternalServerError, "internal_error", "ERROR"},
	}
	for _, tc := range cases {
		buf.Reset()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		var body Error
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != tc.status || body.Code != tc.code || strings.Contains(rec.Body.String(), "password") {
			t.Fatalf("%s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
		var line map[string]any
		if err := json.Unmarshal(buf.Bytes(), &line); err != nil || line["level"] != tc.level {
			t.Fatalf("%s: log %s", tc.path, buf.String())
		}
		if tc.level == "ERROR" && (line["code"] != "internal_error" || !strings.Contains(fmt.Sprint(line["err"]), "password")) {
			t.Fatalf("%s: the cause must reach the log: %s", tc.path, buf.String())
		}
	}
}
