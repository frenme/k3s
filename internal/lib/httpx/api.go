package httpx

import (
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/egor/k3s-base/internal/lib/version"
)

func init() {
	huma.NewError = newAPIError
}

type APIError struct {
	status  int
	Message string `json:"error"`
}

func (e *APIError) Error() string { return e.Message }

func (e *APIError) GetStatus() int { return e.status }

func newAPIError(status int, msg string, errs ...error) huma.StatusError {
	details := make([]string, 0, len(errs))
	for _, err := range errs {
		var detail *huma.ErrorDetail
		switch {
		case err == nil:
		case errors.As(err, &detail) && detail.Location != "":
			details = append(details, detail.Location+": "+detail.Message)
		default:
			details = append(details, err.Error())
		}
	}
	if len(details) > 0 {
		msg += ": " + strings.Join(details, "; ")
	}
	return &APIError{status: status, Message: msg}
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
