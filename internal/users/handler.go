package users

import (
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/google/uuid"
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
	limit, err := httpx.QueryInt(r, "limit", 50, 1, 500)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.store.List(r.Context(), limit)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if uuid.Validate(id) != nil {
		h.respondError(w, r, ErrNotFound)
		return
	}
	u, err := h.store.Get(r.Context(), id)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

type createRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if _, err := mail.ParseAddress(req.Email); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid email")
		return
	}
	if req.Name == "" {
		httpx.WriteError(w, http.StatusBadRequest, "name is required")
		return
	}
	u, err := h.store.Create(r.Context(), req.Email, req.Name)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, u)
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrDuplicate):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "users handler", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
	}
}
