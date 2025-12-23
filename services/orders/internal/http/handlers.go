package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/example/webshop/orders/internal/order"
)

type Handler struct {
	svc *order.Service
}

func NewHandler(svc *order.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Router() *chi.Mux {
	r := chi.NewRouter()
	r.Post("/", h.createOrder)
	r.Get("/", h.listOrders)
	r.Get("/{id}", h.getOrder)
	return r
}

func (h *Handler) createOrder(w http.ResponseWriter, r *http.Request) {
	type req struct {
		UserID      string `json:"user_id"`
		Amount      int64  `json:"amount"`
		Description string `json:"description"`
	}
	var body req
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.UserID == "" || body.Amount <= 0 {
		http.Error(w, "user_id and positive amount required", http.StatusBadRequest)
		return
	}

	order, _, _, err := h.svc.CreateOrder(r.Context(), body.UserID, body.Amount, body.Description)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":          order.ID,
		"user_id":     order.UserID,
		"amount":      order.Amount,
		"description": order.Description,
		"status":      order.Status,
	})
}

func (h *Handler) listOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	items, err := h.svc.ListOrders(r.Context(), userID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}

func (h *Handler) getOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	o, err := h.svc.GetOrder(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(o)
}

