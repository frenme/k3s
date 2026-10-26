package httpx

import (
	"context"
	"net/http"
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
