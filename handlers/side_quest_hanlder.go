package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"time"
)

type SideQuestHandler struct {
	sideQuestService *services.SideQuestService
}

func NewSideQuestHandler(sideQuestService *services.SideQuestService) *SideQuestHandler {
	return &SideQuestHandler{
		sideQuestService: sideQuestService,
	}
}

func (h *SideQuestHandler) GetSideQuestBoard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	board, err := h.sideQuestService.GetSideQuestBoard(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not serve quest board")
		return
	}

	respondWithJSON(w, http.StatusOK, board)

}

func (h *SideQuestHandler) PostNewSideQuest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		Title         string    `json:"title"`
		Description   string    `json:"description"`
		Reward        int       `json:"reward"`
		DurationHours time.Time `json:"duration_hours"`
		IsPublic      bool      `json:"is_public"`
		IsAnonymous   bool      `json:"is_anonymous"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	newPost, err := h.sideQuestService.PostNewSideQuest(ctx, clerkID, req.Title, req.Description, req.Reward, req.DurationHours, req.IsPublic, req.IsAnonymous)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not post new quest")
		return
	}

	respondWithJSON(w, http.StatusOK, newPost)

}
