package handler

import (
	"encoding/json"
	"fsanano/go-test/internal/service"
	"net/http"
)

type ShopHandler struct {
	svc *service.ShopService
}

func NewShopHandler(svc *service.ShopService) *ShopHandler {
	return &ShopHandler{svc: svc}
}

type BuyRequest struct {
	UserID int `json:"user_id"`
	ItemID int `json:"item_id"`
	Count  int `json:"count"` // Optional, defaults to 1 if 0
}

func (h *ShopHandler) BuyItem(w http.ResponseWriter, r *http.Request) {
	var req BuyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Default count to 1 if not provided
	quantity := req.Count
	if quantity <= 0 {
		quantity = 1
	}

	if err := h.svc.BuyItem(r.Context(), req.UserID, req.ItemID, quantity); err != nil {
		if err.Error() == "item not found" || err.Error() == "user not found" || err.Error() == "insufficient funds" || err.Error() == "insufficient stock" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		// Log error internally in production
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "success"}`))
}
