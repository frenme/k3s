package httpx

import (
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/egor/k3s-base/internal/lib/version"
)

func init() {
	huma.NewError = newAPIError
}

func newAPIError(status int, _ string, errs ...error) huma.StatusError {
	e := standard(status)
	if status >= http.StatusInternalServerError {
		return e
	}
	for _, err := range errs {
		var detail *huma.ErrorDetail
		switch {
		case err == nil:
		case errors.As(err, &detail) && detail.Location != "":
			e = e.With(detail.Location + ": " + detail.Message)
		default:
			e = e.With(err.Error())
		}
	}
	return e
}

func NewAPI(mux *http.ServeMux, service string) huma.API {
	cfg := apiConfig(service)
	cfg.Servers = []*huma.Server{{URL: "/api"}}
	cfg.OpenAPIPath = "/v1/" + service + "/openapi"
	cfg.DocsPath = "/v1/" + service + "/docs"
	return humago.New(mux, cfg)
}

func NewInternalAPI(mux *http.ServeMux, service string) huma.API {
	cfg := apiConfig(service + " internal")
	cfg.OpenAPIPath = "/internal/v1/" + service + "/openapi"
	return humago.New(mux, cfg)
}

func apiConfig(title string) huma.Config {
	cfg := huma.DefaultConfig(title, version.Version)
	cfg.CreateHooks = nil
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	return cfg
}
