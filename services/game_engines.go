package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Internal Card: Use Short Codes ("H", "S") to make logic easier
type Card struct {
	Suit string `json:"suit"` 
	Rank string `json:"rank"` 
}

type ClientCard struct {
	Suit     string `json:"suit"`  
	Value    string `json:"value"` 
	Rule     string `json:"rule"`
	Color    string `json:"color"`
	ImageUrl string `json:"imageUrl"` 
}

type GameStatePayload struct {
	Action    string          `json:"action"` 
	GameState ClientGameState `json:"gameState"`
}

type ClientGameState struct {
	CurrentCard    *ClientCard `json:"currentCard"`
	CardsRemaining int         `json:"cardsRemaining"`
	GameOver       bool        `json:"gameOver"`
}

// 1. REVERTED TO SHORT CODES so the helper functions work
func generateNewDeck() []Card {
	suits := []string{"H", "D", "C", "S"} 
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

// --- HELPER FUNCTIONS ---

func getSuitName(s string) string {
	switch s {
	case "H": return "hearts"
	case "D": return "diamonds"
	case "C": return "clubs"
	case "S": return "spades"
	default: return "hearts"
	}
}

func getCardColor(s string) string {
	if s == "H" || s == "D" {
		return "#ef4444" 
	}
	return "black"
}

func getRule(rank string) string {
	switch rank {
	case "A": return "Waterfall - Everyone drinks!"
	case "2": return "You - Pick someone to drink"
	case "3": return "Me - You drink"
	case "4": return "Whores - All girls drink"
	case "5": return "Thumb Master"
	case "6": return "Dicks - All guys drink"
	case "7": return "Heaven - Point to the sky"
	case "8": return "Mate - Pick a drinking buddy"
	case "9": return "Rhyme - Pick a word"
	case "10": return "Categories"
	case "J": return "Never Have I Ever"
	case "Q": return "Question Master"
	case "K": return "Make a Rule"
	default: return "Drink!"
	}
}

func getImageUrl(rank, suit string) string {
	// Map internal "H" to "hearts" for the URL
	fullSuit := getSuitName(suit) 
	
	// Map internal "A" to "ace" if your image naming requires it
	// Assuming your server images are like "A-of-hearts.png"
	
	return fmt.Sprintf("https://outdrinkmeapi-dev.onrender.com/assets/images/cards/%s/%s-of-%s.png", fullSuit, rank, fullSuit)
}

// --- MAIN HANDLER ---

func (g *KingsCupLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	if !sender.IsHost {
		return
	}

	// 2. Parse Action AND Type
	var payload struct {
		Action string `json:"action"` 
		Type   string `json:"type"`   // <--- We read Type here
	}
	json.Unmarshal(msg, &payload)

	// 3. CHECK TYPE, NOT ACTION
	if payload.Type == "draw_card" { 
		g.mu.Lock()

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

		drawn := g.Deck[0]
		g.Deck = g.Deck[1:]
		g.CurrentCard = &drawn
		remaining := len(g.Deck)
		g.mu.Unlock()

		clientCard := ClientCard{
			Suit:     getSuitName(drawn.Suit),
			Value:    drawn.Rank,
			Rule:     getRule(drawn.Rank),
			Color:    getCardColor(drawn.Suit),
			ImageUrl: getImageUrl(drawn.Rank, drawn.Suit), 
		}

		response := GameStatePayload{
			Action: "game_update",
			GameState: ClientGameState{
				CurrentCard:    &clientCard,
				CardsRemaining: remaining,
				GameOver:       false,
			},
		}

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
