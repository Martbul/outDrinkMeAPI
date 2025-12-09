package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
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
	userService *services.UserService
}

func NewDrinkingGamesHandler(gameManager *services.DrinnkingGameManager, userService *services.UserService) *DrinkingGamesHandler {
	return &DrinkingGamesHandler{
		gameManager: gameManager,
		userService: userService,
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

	user, err := h.userService.GetUserByClerkID(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	var req struct {
		GameType string `json:"game_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	sessionID := uuid.New().String()

	h.gameManager.CreateSession(ctx, sessionID, req.GameType, clerkID, user.Username)

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

	session, exists := h.gameManager.GetSession(sessionID)
	if !exists {
		http.Error(w, "Game session not found", http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &services.Client{
		Session: session,
		Conn:    conn,
		Send:    make(chan []byte, 256),
	}

	client.Session.Register <- client
	go client.WritePump()
	go client.ReadPump()

}
