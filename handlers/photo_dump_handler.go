package handlers

import (
	"context"
	"net/http"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"time"
)

type PhotoDumpHandler struct {
	photoDumpService *services.PhotoDumpService
}

func NewPhotoDumpHandler(photoDumpService *services.PhotoDumpService) *PhotoDumpHandler {
	return &PhotoDumpHandler{
		photoDumpService: photoDumpService,
	}
}

func (h *PhotoDumpHandler) GenerateQrCode(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	qrCode, err := h.photoDumpService.GenerateQrCode(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "unable to generate qr code")
		return
	}

	respondWithJSON(w, http.StatusCreated, qrCode)
}

func (h *PhotoDumpHandler) JoinViaQrCode(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	success, err := h.photoDumpService.JoinViaQrCode(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "unable to join")
		return
	}

	respondWithJSON(w, http.StatusOK, success)
}

func (h *PhotoDumpHandler) GetSessionData(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}


	photoDumpSessionData, err := h.photoDumpService.GetSessionData(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "unable to get photo dump session")
		return
	}

	respondWithJSON(w, http.StatusOK, photoDumpSessionData)

}

func (h *PhotoDumpHandler) AddImages(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	countAddedImages, err := h.photoDumpService.AddImages(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "unable to add images to photo dump")
		return
	}

	respondWithJSON(w, http.StatusCreated, countAddedImages)
}
