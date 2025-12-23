package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/example/webshop/payments/internal/account"
)

type Handler struct {
	accounts *account.Repository
}

func NewHandler(accounts *account.Repository) *Handler {
	return &Handler{accounts: accounts}
}

func (h *Handler) Router() *chi.Mux {
	r := chi.NewRouter()
	r.Post("/accounts", h.createAccount)
	r.Post("/accounts/deposit", h.deposit)
	r.Get("/accounts/{user_id}/balance", h.balance)
	return r
}

func (h *Handler) createAccount(w http.ResponseWriter, r *http.Request) {
	type req struct {
		UserID string `json:"user_id"`
	}
	var body req
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.UserID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}
	created, err := h.accounts.CreateIfAbsent(r.Context(), body.UserID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	if !created {
		http.Error(w, "account exists", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) deposit(w http.ResponseWriter, r *http.Request) {
	type req struct {
		UserID string `json:"user_id"`
		Amount int64  `json:"amount"`
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
	balance, err := h.accounts.Deposit(r.Context(), body.UserID, body.Amount)
	if err != nil {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user_id": body.UserID,
		"balance": balance,
	})
}

func (h *Handler) balance(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "user_id")
	balance, err := h.accounts.Balance(r.Context(), userID)
	if err != nil {
		http.Error(w, "account not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"user_id": userID,
		"balance": balance,
	})
}

