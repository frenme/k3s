package httpx

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

type Error struct {
	Status  int      `json:"-"`
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

func (e Error) Error() string { return e.Code + ": " + e.Message }

func (e Error) GetStatus() int { return e.Status }

func (e Error) Is(target error) bool {
	t, ok := target.(Error)
	return ok && t.Code == e.Code
}

func (e Error) With(details ...string) Error {
	e.Details = append(append([]string(nil), e.Details...), details...)
	return e
}

var (
	ErrBadRequest       = Error{Status: http.StatusBadRequest, Code: "bad_request", Message: "Malformed request"}
	ErrUnauthorized     = Error{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Valid access token required"}
	ErrNotFound         = Error{Status: http.StatusNotFound, Code: "not_found", Message: "Not found"}
	ErrMethodNotAllowed = Error{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "Method not allowed"}
	ErrInvalidInput     = Error{Status: http.StatusUnprocessableEntity, Code: "invalid_input", Message: "Invalid input"}
	ErrInternal         = Error{Status: http.StatusInternalServerError, Code: "internal_error", Message: "Something went wrong"}

	ErrInvalidCursor        = Error{Status: http.StatusUnprocessableEntity, Code: "invalid_cursor", Message: "Invalid cursor"}
	ErrIdempotencyKeyReused = Error{Status: http.StatusUnprocessableEntity, Code: "idempotency_key_reused", Message: "Idempotency-Key was already used with a different request"}
	ErrUnavailable          = Error{Status: http.StatusBadGateway, Code: "upstream_unavailable", Message: "Service is temporarily unavailable"}
)

func standard(status int) Error {
	switch status {
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusMethodNotAllowed:
		return ErrMethodNotAllowed
	case http.StatusUnprocessableEntity:
		return ErrInvalidInput
	case http.StatusInternalServerError:
		return ErrInternal
	}
	if status >= http.StatusInternalServerError {
		return Error{Status: status, Code: "internal_error", Message: ErrInternal.Message}
	}
	return Error{Status: status, Code: "bad_request", Message: http.StatusText(status)}
}

func WriteError(w http.ResponseWriter, e Error) {
	WriteJSON(w, e.Status, e)
}

func fail(ctx context.Context, err error) error {
	var e Error
	if !errors.As(err, &e) {
		e = ErrInternal
	}
	if e.Status >= http.StatusInternalServerError {
		LogAttrs(ctx, slog.String("code", e.Code), slog.Any("err", err))
		markFailed(ctx)
	}
	return e
}
