package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"outDrinkMeAPI/internal/store"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"time"
)

type StoreHandler struct {
	storeService *services.StoreService
}

func NewStoreHandler(storeService *services.StoreService) *StoreHandler {
	return &StoreHandler{
		storeService: storeService,
	}
}

func (h *StoreHandler) GetStore(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	store, err := h.storeService.GetStore(ctx)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "store isnt available")
		return
	}

	respondWithJSON(w, http.StatusOK, store)
}

func (h *StoreHandler) PurchaseStoreItem(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req store.PurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.storeService.PurchaseStoreItem(ctx, clerkID, req.ItemID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}
