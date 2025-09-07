package users

import (
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/egor/k3s-base/internal/platform/httpx"
)

type Handler struct {
	store Store
	log   *slog.Logger
}

func NewHandler(store Store, log *slog.Logger) *Handler {
	return &Handler{store: store, log: log}
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /users", h.list)
	mux.HandleFunc("POST /users", h.create)
	mux.HandleFunc("GET /users/{id}", h.get)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	items, err := h.store.List(r.Context(), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

