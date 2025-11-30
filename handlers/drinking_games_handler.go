package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Could not upgrade cinnection: %v", err)

		return
	}
	defer ws.Close()
}

type DrinkingGamesHandler struct {
	gameManager *services.DrinnkingGameManager
}

func NewDrinkingGamesHandler(gameManager *services.DrinnkingGameManager) *DrinkingGamesHandler {
	return &DrinkingGamesHandler{
		gameManager: gameManager,
	}
}

func (h *DrinkingGamesHandler) CreateDrinkingGame(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Structure matches the frontend JSON
	var req struct {
		GameType string `json:"game_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	sessionID := uuid.New().String()

	// Pass the decoded settings to the manager
	h.gameManager.CreateSession(ctx, sessionID, req.GameType, clerkID)

	response := map[string]string{
		"sessionId": sessionID,
		"wsUrl":     "/api/v1/games/ws/" + sessionID,
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *DrinkingGamesHandler) GetPublicDrinkingGames(w http.ResponseWriter, r *http.Request) {
	games := h.gameManager.GetPublicSessions()

	respondWithJSON(w, http.StatusOK, games)
}

func (h *DrinkingGamesHandler) JoinDrinkingGame(w http.ResponseWriter, r *http.Request) {
	// ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	// defer cancel()

	// token := r.URL.Query().Get("token")
	// if token == "" {
	// 	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	// 	return
	// }

	vars := mux.Vars(r)
	sessionID := vars["sessionID"]

	// 1. Validate Session Exists
	session, exists := h.gameManager.GetSession(sessionID)
	if !exists {
		http.Error(w, "Game session not found", http.StatusNotFound)
		return
	}

	// 2. Upgrade Connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// 3. Register User to Session
	client := &services.Client{
		Session: session,
		Conn:    conn,
		Send:    make(chan []byte, 256),
	}

	// Start Pumps
	client.Session.Register <- client
	go client.WritePump()
	go client.ReadPump()

}
