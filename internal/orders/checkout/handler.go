package checkout

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/orders/model"
)

type Handler struct {
	svc *Service
	log *slog.Logger
}

func NewHandler(svc *Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Mount(public *http.ServeMux) {
	public.HandleFunc("GET /orders", h.list)
	public.HandleFunc("POST /orders", h.create)
	public.HandleFunc("GET /orders/{id}", h.get)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := httpx.QueryInt(r, "limit", 50, 1, 500)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	items, err := h.svc.List(r.Context(), r.URL.Query().Get("user_id"), limit)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	o, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

type createRequest struct {
	UserID      string `json:"user_id"`
	Item        string `json:"item"`
	AmountCents int64  `json:"amount_cents"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid json")
		return
	}
	order, created, err := h.svc.Create(r.Context(), model.Order{
		UserID:      req.UserID,
		Item:        req.Item,
		AmountCents: req.AmountCents,
	}, r.Header.Get("Idempotency-Key"))
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	httpx.WriteJSON(w, status, order)
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid model.ValidationError
	switch {
	case errors.As(err, &invalid):
		httpx.WriteError(w, http.StatusBadRequest, invalid.Error())
	case errors.Is(err, model.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, model.ErrUnknownUser):
		httpx.WriteError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, errUsersUnavailable):
		h.log.ErrorContext(r.Context(), "users lookup", "err", err)
		httpx.WriteError(w, http.StatusBadGateway, errUsersUnavailable.Error())
	default:
		h.log.ErrorContext(r.Context(), "checkout handler", "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
	}
}
