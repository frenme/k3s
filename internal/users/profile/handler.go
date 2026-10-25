package profile

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/users/model"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Mount(public, internal *http.ServeMux) {
	public.HandleFunc("GET /users", h.list)
	public.HandleFunc("POST /users", h.create)
	public.HandleFunc("GET /users/{id}", h.get)
	internal.HandleFunc("GET /internal/users/{id}", h.get)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := httpx.QueryInt(r, "limit", 50, 1, 500)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.svc.List(r.Context(), limit)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.Get(r.Context(), r.PathValue("id"))
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
	u, err := h.svc.Create(r.Context(), req.Email, req.Name)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, u)
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid model.ValidationError
	switch {
	case errors.As(err, &invalid):
		httpx.WriteError(w, http.StatusBadRequest, invalid.Error())
	case errors.Is(err, model.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, model.ErrDuplicate):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	default:
		h.log.ErrorContext(r.Context(), "profile handler", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
	}
}
