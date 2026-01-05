package handlers

import (
	"context"
	"encoding/json"
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

func (h *VenueHandler) GetEmployeeDetails(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	allVenues, err := h.venueService.GetEmployeeDetails(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Server error")
		return
	}

	respondWithJSON(w, http.StatusOK, allVenues)
}

type AddEmployeeRequest struct {
	VenueID string `json:"venueId"`
	Role    string `json:"role"`
}

type RemoveEmployeeRequest struct {
	VenueID string `json:"venueId"`
}

func (h *VenueHandler) AddEmployeeToVenue(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req AddEmployeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.VenueID == "" || req.Role == "" {
		respondWithError(w, http.StatusBadRequest, "venueId and role are required")
		return
	}

	success, err := h.venueService.AddEmployeeToVenue(ctx, req.VenueID, clerkID, req.Role)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Server error")
		return
	}

	respondWithJSON(w, http.StatusOK, success)
}

func (h *VenueHandler) RemoveEmployeeFromVenue(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req RemoveEmployeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.VenueID == "" {
		respondWithError(w, http.StatusBadRequest, "venueId is required")
		return
	}

	success, err := h.venueService.RemoveEmployeeFromVenue(ctx, req.VenueID, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Server error")
		return
	}

	respondWithJSON(w, http.StatusOK, success)
}

func (h *VenueHandler) AddScanData(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		DiscountPercentage string `json:"discount_percentage"`
		VenueID            string `json:"venue"` 
		ScannerUserId      string `json:"scanner_user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.VenueID == "" || req.ScannerUserId == "" {
		respondWithError(w, http.StatusBadRequest, "Venue and ScannerUserId are required")
		return
	}

	serviceReq := services.ScanDataReq{
		VenueID:            req.VenueID,
		CustomerID:         clerkID,          
		ScannerUserID:      req.ScannerUserId, 
		DiscountPercentage: req.DiscountPercentage,
	}

	success, err := h.venueService.AddScanData(ctx, serviceReq)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Server error: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, success)
}