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

