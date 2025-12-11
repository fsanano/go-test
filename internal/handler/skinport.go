package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"fsanano/go-test/internal/service/skinport"
)

func (h *Handler) GetSkinportItems(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	currency := r.URL.Query().Get("currency")

	// Pass the context from the request
	items, err := h.skinportClient.GetAllItems(r.Context(), appID, currency)
	if err != nil {
		fmt.Printf("Error fetching items: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)

		var apiErr *skinport.ErrorResponse
		if errors.As(err, &apiErr) {
			json.NewEncoder(w).Encode(apiErr)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(items); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
