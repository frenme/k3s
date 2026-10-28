package validate

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/egor/k3s-base/internal/gateway/model"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

type Validator struct {
	router routers.Router
}

var formats sync.Once

func New(spec []byte) (*Validator, error) {
	formats.Do(func() {
		openapi3.DefineStringFormatValidator("email", openapi3.NewRegexpFormatValidator(openapi3.FormatOfStringForEmail))
		openapi3.DefineStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(openapi3.FormatOfStringForUUIDOfRFC4122))
	})
	doc, err := openapi3.NewLoader().LoadFromData(spec)
	if err != nil {
		return nil, fmt.Errorf("load openapi: %w", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("build openapi router: %w", err)
	}
	return &Validator{router: router}, nil
}

func (v *Validator) Middleware() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, params, err := v.router.FindRoute(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			err = openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: params,
				Route:      route,
				Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
			})
			if err != nil {
				httpx.WriteError(w, toError(err))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func toError(err error) httpx.Error {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return model.ErrPayloadTooLarge
	}
	var reqErr *openapi3filter.RequestError
	if !errors.As(err, &reqErr) {
		return httpx.ErrInvalidInput.With(firstLine(err.Error()))
	}
	var parseErr *openapi3filter.ParseError
	if reqErr.RequestBody != nil && errors.As(reqErr.Err, &parseErr) {
		return httpx.ErrBadRequest
	}
	reason := reqErr.Reason
	if reason == "" && reqErr.Err != nil {
		reason = firstLine(reqErr.Err.Error())
	}
	switch {
	case reqErr.Parameter != nil:
		return httpx.ErrInvalidInput.With(fmt.Sprintf("%s.%s: %s", reqErr.Parameter.In, reqErr.Parameter.Name, reason))
	case reqErr.RequestBody != nil:
		return httpx.ErrInvalidInput.With("body: " + reason)
	}
	return httpx.ErrInvalidInput.With(reason)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
