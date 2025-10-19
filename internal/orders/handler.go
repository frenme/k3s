package orders

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/egor/k3s-base/internal/platform/httpx"
)

type Handler struct {
	store Store
	users UserChecker
	log   *slog.Logger
}

func NewHandler(store Store, users UserChecker, log *slog.Logger) *Handler {
	return &Handler{store: store, users: users, log: log}
}

func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /orders", h.list)
	mux.HandleFunc("POST /orders", h.create)
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
	items, err := h.store.List(r.Context(), r.URL.Query().Get("user_id"), limit)
	if err != nil {
		h.internal(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

