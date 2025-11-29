package handlers

import (
	"context"
	"log"
	"net/http"
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

	vars := mux.Vars(r)
	gameType := vars["gameType"]

	sessionID := uuid.New().String()

	h.gameManager.CreateSession(ctx, sessionID, gameType)

	response := map[string]string{
		"sessionId": sessionID,
		"wsUrl":     "/api/v1/games/ws/" + sessionID,
	}

	respondWithJSON(w, http.StatusOK, response)

}

func (h *DrinkingGamesHandler) JoinDrinkingGame(w http.ResponseWriter, r *http.Request) {
	// ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	// defer cancel()

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
