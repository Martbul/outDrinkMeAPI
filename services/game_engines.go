package services

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// Internal Card Representation
type Card struct {
	Suit string `json:"suit"` // "H", "D", "C", "S"
	Rank string `json:"rank"` // "A", "2"..."10", "J", "Q", "K"
}

// Client-Facing Card Representation (Matches React Native interfaces)
type ClientCard struct {
	Suit     string `json:"suit"`  // "hearts", etc
	Value    string `json:"value"` // "A", "K"
	Rule     string `json:"rule"`
	Color    string `json:"color"`
	ImageUrl string `json:"imageUrl"` // <--- NEW: The requested image URL
}

type GameStatePayload struct {
	Action    string          `json:"action"` // "game_update"
	GameState ClientGameState `json:"gameState"`
}

type ClientGameState struct {
	CurrentCard    *ClientCard `json:"currentCard"`
	CardsRemaining int         `json:"cardsRemaining"`
	GameOver       bool        `json:"gameOver"`
}

func generateNewDeck() []Card {
	suits := []string{"hearts", "diamonds", "clubs", "spades"}
	ranks := []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}
	deck := make([]Card, 0, 52)
	for _, s := range suits {
		for _, r := range ranks {
			deck = append(deck, Card{Suit: s, Rank: r})
		}
	}
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)
	r.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
	return deck
}

type KingsCupLogic struct {
	Deck        []Card
	CurrentCard *Card
	mu          sync.Mutex
}

func (g *KingsCupLogic) InitState() interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Deck = generateNewDeck()
	g.CurrentCard = nil
	return nil
}

// --- HELPER FUNCTIONS FOR RULES & IMAGES ---

func getSuitName(s string) string {
	switch s {
	case "H":
		return "hearts"
	case "D":
		return "diamonds"
	case "C":
		return "clubs"
	case "S":
		return "spades"
	default:
		return "hearts"
	}
}

func getCardColor(s string) string {
	if s == "H" || s == "D" {
		return "#ef4444" // Red
	}
	return "black"
}

// Simple rule mapping for Kings Cup
func getRule(rank string) string {
	switch rank {
	case "A":
		return "Waterfall - Everyone drinks!"
	case "2":
		return "You - Pick someone to drink"
	case "3":
		return "Me - You drink"
	case "4":
		return "Whores - All girls drink"
	case "5":
		return "Thumb Master"
	case "6":
		return "Dicks - All guys drink"
	case "7":
		return "Heaven - Point to the sky"
	case "8":
		return "Mate - Pick a drinking buddy"
	case "9":
		return "Rhyme - Pick a word"
	case "10":
		return "Categories"
	case "J":
		return "Never Have I Ever"
	case "Q":
		return "Question Master"
	case "K":
		return "Make a Rule"
	default:
		return "Drink!"
	}
}

// Generates a URL from the deckofcardsapi CDN
func getImageUrl(rank, suit string) string {
	// API format: 0=10, H=Hearts, etc.
	apiRank := rank
	if rank == "10" {
		apiRank = "0"
	}

	// return fmt.Sprintf("https://deckofcardsapi.com/static/img/%s%s.png", apiRank, suit)
	return fmt.Sprintf("	https://outdrinkmeapi-dev.onrender.com/assets/images/cards/%s/%s-of-%s.png", suit, apiRank, suit)
}

// --- MAIN HANDLER ---

func (g *KingsCupLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	// 1. Only Host can draw cards
	if !sender.IsHost {
		return
	}

	// 2. Parse Action (Double check it is draw_card, though Client already checks)
	var payload struct {
		Action string `json:"action"` // "draw_card" or "game_action"
		Type   string `json:"type"`   // "draw_card" (if using nested action)
	}
	
	json.Unmarshal(msg, &payload)

	if payload.Action == "draw_card" {
		g.mu.Lock()

		log.Println(g.Deck)
		// Game Over Check
		if len(g.Deck) == 0 {
			g.mu.Unlock()
			response := GameStatePayload{
				Action: "game_update",
				GameState: ClientGameState{
					CurrentCard:    nil,
					CardsRemaining: 0,
					GameOver:       true,
				},
			}
			bytes, _ := json.Marshal(response)
			s.Broadcast <- bytes
			return
		}

		// Draw logic
		drawn := g.Deck[0]
		g.Deck = g.Deck[1:]
		g.CurrentCard = &drawn
		remaining := len(g.Deck)
		g.mu.Unlock()

		// 3. Transform to Client Data
		clientCard := ClientCard{
			Suit:     getSuitName(drawn.Suit),
			Value:    drawn.Rank,
			Rule:     getRule(drawn.Rank),
			Color:    getCardColor(drawn.Suit),
			ImageUrl: getImageUrl(drawn.Rank, drawn.Suit), // <--- Here is your URL
		}

		// 4. Construct Payload
		response := GameStatePayload{
			Action: "game_update",
			GameState: ClientGameState{
				CurrentCard:    &clientCard,
				CardsRemaining: remaining,
				GameOver:       false,
			},
		}

		// 5. Broadcast to ALL clients
		bytes, _ := json.Marshal(response)
		s.Broadcast <- bytes
	}
}

type BurnBookLogic struct{}

func (g *BurnBookLogic) InitState() interface{} {
	return nil
}

func (g *BurnBookLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	s.Broadcast <- msg
}
