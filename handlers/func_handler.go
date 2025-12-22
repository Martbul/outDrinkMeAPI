package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"time"

	"github.com/gorilla/mux"
)

type FuncHandler struct {
	funcService *services.FuncService
}

func NewFuncHandler(funcService *services.FuncService) *FuncHandler {
	return &FuncHandler{
		funcService: funcService,
	}
}
func (h *FuncHandler) CreateFunction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	qrCodeResponse, err := h.funcService.GenerateQrCode(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, qrCodeResponse)
}

func (h *FuncHandler) JoinViaQrCode(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var body struct {
		QRToken string `json:"qrToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	funcID, err := h.funcService.JoinViaQrCode(ctx, clerkID, body.QRToken)
	if err != nil {
		respondWithError(w, http.StatusForbidden, "Unable to join: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"funcId": funcID.String()})
}

func (h *FuncHandler) GetSessionData(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	vars := mux.Vars(r)
	funcID := vars["id"]

	if funcID == "" {
		respondWithError(w, http.StatusBadRequest, "Function ID is required")
		return
	}

	photoDumpSessionData, err := h.funcService.GetSessionData(ctx, funcID, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unable to get session: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, photoDumpSessionData)
}


func (h *FuncHandler) AddImages(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var body struct {
		FuncID   string `json:"funcId"`
		ImageURL string `json:"imageUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.funcService.AddImages(ctx, clerkID, body.FuncID, body.ImageURL)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to add image")
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]string{"message": "image added successfully"})
}


func (h *FuncHandler) GetUserActiveSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userActiveSession, err := h.funcService.GetUserActiveSession(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Unable to get session: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, userActiveSession)
}