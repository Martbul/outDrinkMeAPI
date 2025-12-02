// Session handles networking(sending messages), game engine hanlders the rules of the game, manager - destroying the seesion when t is empty,
// register chan - When a user connects via WebSocket, they aren't added to the Clients map immediately. They are pushed into this channel. The Session's Run() loop picks them up one by one and safely adds them to the map.,
// unregiesr - the oposite of register
// broadcast chan - > you put message into it, then run() pics it up and sends it to every client
package services

import (
	"context"
	"encoding/json"
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

// --- 2. The Game Logic Interface (Strategy Pattern) ---
type GameLogic interface {
	HandleMessage(session *Session, sender *Client, message []byte)
	InitState() interface{}
}

type Session struct {
	ID          string
	HostID      string
	GameType    string
	GameEngine  GameLogic
	Manager     *DrinnkingGameManager
	Clients     map[*Client]bool
	Broadcast   chan []byte
	Register    chan *Client
	Unregister  chan *Client
	TriggerList chan bool
}

func NewGameLogic(gameType string) GameLogic {
	switch gameType {
	case "kings-cup":
		return &KingsCupLogic{}
	case "burn-book":
		return &BurnBookLogic{}
	case "mafia":
		return &MafiaLogic{}
	default:
		return &KingsCupLogic{}
	}
}

func NewSession(id, gameType, hostID string, manager *DrinnkingGameManager) *Session {
	return &Session{
		ID:          id,
		HostID:      hostID,
		GameType:    gameType,
		GameEngine:  NewGameLogic(gameType),
		Manager:     manager,
		Clients:     make(map[*Client]bool),
		Broadcast:   make(chan []byte),
		Register:    make(chan *Client),
		Unregister:  make(chan *Client),
		TriggerList: make(chan bool),
	}
}
func (s *Session) sendPlayerListToAll() {
	type PlayerInfo struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		IsHost   bool   `json:"isHost"`
	}

	players := []PlayerInfo{}

	// Safe to read s.Clients here because this function is called inside Run()
	for client := range s.Clients {
		if client.Username != "" {
			players = append(players, PlayerInfo{
				ID:       client.UserID,
				Username: client.Username,
				IsHost:   client.IsHost,
			})
		}
	}

	payload := map[string]interface{}{
		"action":  "update_player_list",
		"players": players,
	}

	data, _ := json.Marshal(payload)

	// Send to everyone directly
	for client := range s.Clients {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(s.Clients, client)
		}
	}
}

func (s *Session) Run() {
	defer func() {
		close(s.Broadcast)
		close(s.Register)
		close(s.Unregister)
		close(s.TriggerList)
	}()
//! when client disconnects againghe should be unregistered
	for {
		select {
		case client := <-s.Register:
			s.Clients[client] = true
			log.Printf("[Session %s] User connected. Count: %d", s.ID, len(s.Clients))

		case <-s.TriggerList:
			s.sendPlayerListToAll()

		case client := <-s.Unregister:
			if _, ok := s.Clients[client]; ok {
				delete(s.Clients, client)
				close(client.Send)

				// If empty, delete session
				if len(s.Clients) == 0 {
					log.Printf("[Session %s] Empty, destroying.", s.ID)
					s.Manager.DeleteSession(s.ID)
					return
				}
				// If people remain, update their list
				s.sendPlayerListToAll()
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

func (m *DrinnkingGameManager) CreateSession(ctx context.Context, sessionID, gameType, clerkId string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		return s
	}

	s := NewSession(sessionID, gameType, clerkId, m)
	m.sessions[sessionID] = s
	go s.Run()
	return s
}

func (m *DrinnkingGameManager) GetSession(sessionID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	return s, ok
}

type PublicGameResponse struct {
	SessionID string `json:"sessionId"`
	GameType  string `json:"gameType"`
	HostID    string `json:"host"`
	Players   int    `json:"players"`
}

func (m *DrinnkingGameManager) GetPublicSessions() []PublicGameResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Initialize as empty slice so it returns [] instead of null in JSON
	games := make([]PublicGameResponse, 0)

	for _, s := range m.sessions {
		// Optional: Check if s.Settings.IsPublic is true
		// if !s.Settings.IsPublic { continue }

		games = append(games, PublicGameResponse{
			SessionID: s.ID,
			GameType:  s.GameType,
			HostID:    s.HostID,
			Players:   len(s.Clients), // Thread-safe read?
			// Note: Reading len(s.Clients) here is technically a race condition
			// if you don't lock the Session itself, but for a simple list it's usually acceptable.
			// Ideally, Session should have its own RWMutex for its Client map.
		})
	}

	return games
}
func (m *DrinnkingGameManager) DeleteSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// client is like a midlewman between the websocken and the hub
type Client struct {
	Session  *Session
	Conn     *websocket.Conn
	Send     chan []byte
	UserID   string
	Username string
	IsHost   bool
}

type WsPayload struct {
	Action   string `json:"action"`
	Type     string `json:"type"`
	Username string `json:"username"`
	UserID   string `json:"userId"`
	IsHost   bool   `json:"isHost"`
	Content  string `json:"content"`
}

func (c *Client) ReadPump() {
	defer func() {
		c.Session.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			log.Println("herere")
			log.Println("Error reading msg", err)
			break
		}

		var payload WsPayload
		if err := json.Unmarshal(message, &payload); err == nil {
			if payload.Action == "join_room" {
				c.Username = payload.Username
				c.UserID = payload.UserID
				c.IsHost = payload.IsHost
				c.Session.Broadcast <- message
				c.Session.TriggerList <- true
				continue
			}

			if payload.Action == "start_game" {
				c.Session.GameEngine.InitState() // this is sing the strategy patters, so that if in the create part it has been seleceted 1 game that same game's init will be executed here
				// Also broadcast that game started so UI changes to game view
				c.Session.Broadcast <- message
				continue
			}
			

			// We check if it is a game_action. The Engine will check the "Type" (draw_card).
			if payload.Action == "game_action" {
				log.Println("DEBUG: in game_action")
				c.Session.GameEngine.HandleMessage(c.Session, c, message)
				continue
			}
		}
		// Chat messages
		c.Session.Broadcast <- message
	}
}

func (s *Session) getPlayersList() []PlayerInfo {
	players := []PlayerInfo{}
	for client := range s.Clients {
		if client.Username != "" {
			players = append(players, PlayerInfo{
				ID:       client.UserID,
				Username: client.Username,
			})
		}
	}
	return players
}


func (s *Session) BroadcastPlayerList() {
	// create a list of players
	type PlayerInfo struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		IsHost   bool   `json:"isHost"`
	}

	players := []PlayerInfo{}

	// Iterate over clients map
	for client := range s.Clients {
		// Only add clients who have actually completed the join handshake (have a username)
		if client.Username != "" {
			players = append(players, PlayerInfo{
				ID:       client.UserID,
				Username: client.Username,
				IsHost:   client.IsHost,
			})
		}
	}

	payload := map[string]interface{}{
		"action":  "update_player_list",
		"players": players,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshalling player list:", err)
		return
	}

	s.Broadcast <- data
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
