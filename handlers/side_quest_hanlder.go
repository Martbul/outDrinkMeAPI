package handlers

import (
	"context"
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

func (h *SideQuestHandler) GetBuddiesSideQuestBoard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	buddiesQuestBoard, err := h.sideQuestService.GetBuddiesSideQuestBoard(clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not server buddies quest board")
		return
	}

	respondWithJSON(w, http.StatusOK, buddiesQuestBoard)

}
