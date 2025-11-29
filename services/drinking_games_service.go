package services

import (
	"context"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Session struct {
	ID       string
	GameType string // "poker", "racing", "trivia"
	Manager  *DrinnkingGameManager

	// Registered clients in this specific session
	Clients    map[*Client]bool
	Broadcast  chan []byte
	Register   chan *Client
	Unregister chan *Client
}

func NewSession(id, gameType string, manager *DrinnkingGameManager) *Session {
	return &Session{
		ID:         id,
		GameType:   gameType,
		Manager:    manager,
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (s *Session) Run() {
	defer func() {
		// Cleanup when the loop ends
		close(s.Broadcast)
		close(s.Register)
		close(s.Unregister)
	}()

	for {
		select {
		case client := <-s.Register:
			s.Clients[client] = true
			log.Printf("[Session %s] User joined. Count: %d", s.ID, len(s.Clients))

		case client := <-s.Unregister:
			if _, ok := s.Clients[client]; ok {
				delete(s.Clients, client)
				close(client.Send)
			}
			// If session is empty, remove it from the manager to save memory
			if len(s.Clients) == 0 {
				log.Printf("[Session %s] Empty, destroying session.", s.ID)
				s.Manager.DeleteSession(s.ID)
				return // Stop this goroutine
			}

		case message := <-s.Broadcast:
			for client := range s.Clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(s.Clients, client)
				}
			}
		}
	}

}

// The Manager holds all active games
type DrinnkingGameManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewDrinnkingGameManager() *DrinnkingGameManager {
	return &DrinnkingGameManager{
		sessions: make(map[string]*Session),
	}
}

func (m *DrinnkingGameManager) CreateSession(ctx context.Context ,sessionID, gameType string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		return s
	}

	s := NewSession(sessionID, gameType, m)
	m.sessions[sessionID] = s
	go s.Run() // Start the session's background worker
	return s
}

func (m *DrinnkingGameManager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	return s, ok
}

func (m *DrinnkingGameManager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// client is like a midlewman between the websocken and the hub
type Client struct {
	Session *Session
	Conn    *websocket.Conn
	Send    chan []byte
}


func (c *Client) ReadPump() {
	defer func() {
		c.Session.Unregister <- c
		c.Conn.Close()
	}()
	// ... (Standard read limits and pong handlers here) ...
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		// Broadcast to others in the room
		c.Session.Broadcast <- message
	}
}


func (c *Client) WritePump() {
	// ... (Standard write logic with tickers here) ...
	// refer to previous answer for full implementation
	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}