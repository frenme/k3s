package orders

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/google/uuid"
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
	mux.HandleFunc("GET /orders/{id}", h.get)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := httpx.QueryInt(r, "limit", 50, 1, 500)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := r.URL.Query().Get("user_id")
	if userID != "" && uuid.Validate(userID) != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}
	items, err := h.store.List(r.Context(), userID, limit)
	if err != nil {
		h.respondError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if uuid.Validate(id) != nil {
		httpx.WriteError(w, http.StatusNotFound, ErrNotFound.Error())
		return
	}
	o, err := h.store.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
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
	req.Item = strings.TrimSpace(req.Item)
	if req.UserID == "" || req.Item == "" || req.AmountCents < 0 {
		httpx.WriteError(w, http.StatusBadRequest, "user_id, item and non-negative amount_cents are required")
		return
	}
	if uuid.Validate(req.UserID) != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid user_id")
		return
	}
	ok, err := h.users.Exists(r.Context(), req.UserID)
	if err != nil {
		h.log.Error("users lookup", "err", err, "request_id", httpx.RequestIDFrom(r.Context()))
		httpx.WriteError(w, http.StatusBadGateway, "users service unavailable")
		return
	}
	if !ok {
		httpx.WriteError(w, http.StatusUnprocessableEntity, "user does not exist")
		return
	}
	order, created, err := h.store.Create(r.Context(), Order{
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
	h.log.Error("orders handler", "err", err, "request_id", httpx.RequestIDFrom(r.Context()))
	httpx.WriteError(w, http.StatusInternalServerError, "internal error")
}
