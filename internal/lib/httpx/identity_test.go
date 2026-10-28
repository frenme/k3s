package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

type whoami struct {
	Body struct {
		ID string `json:"id"`
	}
}

func TestCurrentUserComesFromTheGatewayHeader(t *testing.T) {
	mux := http.NewServeMux()
	handler := func(ctx context.Context, _ *struct{}) (*whoami, error) {
		id, err := CurrentUser(ctx)
		if err != nil {
			return nil, err
		}
		out := &whoami{}
		out.Body.ID = id
		return out, nil
	}
	Register(NewAPI(mux, "demo"), huma.Operation{OperationID: "me", Method: http.MethodGet, Path: "/v1/demo/me"}, AccessUser, handler)
	RegisterInternal(NewInternalAPI(mux, "demo"), huma.Operation{OperationID: "internal-me", Method: http.MethodGet, Path: "/internal/v1/demo/me"}, handler)

	cases := []struct {
		name, path, header string
		status             int
		id                 string
	}{
		{"public with the header", "/v1/demo/me", "user-1", http.StatusOK, "user-1"},
		{"public without the header", "/v1/demo/me", "", http.StatusUnauthorized, ""},
		{"internal calls carry no user", "/internal/v1/demo/me", "user-1", http.StatusUnauthorized, ""},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		if c.header != "" {
			req.Header.Set(UserIDHeader, c.header)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		var body struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if rec.Code != c.status || body.ID != c.id {
			t.Fatalf("%s: %d %s", c.name, rec.Code, rec.Body.String())
		}
		if c.status == http.StatusUnauthorized && body.Code != ErrUnauthorized.Code {
			t.Fatalf("%s: code %q", c.name, body.Code)
		}
	}
}
