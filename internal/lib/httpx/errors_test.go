package httpx

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

var errThingNotFound = Error{Status: http.StatusNotFound, Code: "thing_not_found", Message: "Thing not found"}

func TestFail(t *testing.T) {
	cases := []struct {
		in   error
		want Error
	}{
		{errThingNotFound, errThingNotFound},
		{fmt.Errorf("load thing 7: %w", errThingNotFound), errThingNotFound},
		{errors.New("pq: connection reset"), ErrInternal},
	}
	for _, tc := range cases {
		var got Error
		if !errors.As(fail(context.Background(), tc.in), &got) || got.Code != tc.want.Code || got.Status != tc.want.Status || len(got.Details) != 0 {
			t.Fatalf("%v: got %+v", tc.in, got)
		}
	}
}

func TestErrorsMatchByCode(t *testing.T) {
	if !errors.Is(fmt.Errorf("wrap: %w", errThingNotFound.With("id 7")), errThingNotFound) {
		t.Fatal("same code must match")
	}
	if errors.Is(errThingNotFound, ErrNotFound) {
		t.Fatal("different codes must not match")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, errThingNotFound.With("id 7"))
	if rec.Code != http.StatusNotFound || strings.TrimSpace(rec.Body.String()) != `{"code":"thing_not_found","message":"Thing not found","details":["id 7"]}` {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestHumaErrorsUseTheErrorFormat(t *testing.T) {
	e := huma.NewError(http.StatusUnprocessableEntity, "validation failed", &huma.ErrorDetail{Location: "body.email", Message: "expected string"})
	var got Error
	if !errors.As(e, &got) || got.Code != "invalid_input" || len(got.Details) != 1 || got.Details[0] != "body.email: expected string" {
		t.Fatalf("got %+v", e)
	}
	leak := huma.NewError(http.StatusInternalServerError, "unexpected error occurred", errors.New("pq: password authentication failed"))
	if !errors.As(leak, &got) || got.Message != ErrInternal.Message || len(got.Details) != 0 {
		t.Fatalf("internal error leaks: %+v", leak)
	}
}
