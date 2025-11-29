package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
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
	
	// Standard configuration to ensure connection health
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { 
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil 
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS Error: %v", err)
			}
			break
		}
		// Send message to the session to be broadcasted
		c.Session.Broadcast <- message
	}
}

// WritePump handles messages going TO the frontend
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The session closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			// Heartbeat: keep connection alive
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}