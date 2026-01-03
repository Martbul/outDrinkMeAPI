package handlers

import (
	"context"
	"net/http"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"time"
)

type VenueHandler struct {
	venueService *services.VenueService
}

func NewVenueHandler(venueService *services.VenueService) *VenueHandler {
	return &VenueHandler{
		venueService: venueService,
	}
}

func (h *VenueHandler) GetAllVenues(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	_, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	allVenues, err := h.venueService.GetAllVenues(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Server error")
		return
	}

	respondWithJSON(w, http.StatusOK, allVenues)
}
