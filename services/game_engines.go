package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Internal Card: Use Short Codes ("H", "A") internally
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

// 1. GENERATE DECK: Use Short Codes (H, D, C, S) and (A, 2-10, J, Q, K)
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

// 2. HELPER: Map Short Suit ("H") to Long Name ("hearts")
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

// 3. HELPER: Map Short Rank ("A") to Long Name ("ace") for URL generation
func getRankName(r string) string {
	switch r {
	case "A":
		return "ace"
	case "2":
		return "two"
	case "3":
		return "three"
	case "4":
		return "four"
	case "5":
		return "five"
	case "6":
		return "six"
	case "7":
		return "seven"
	case "8":
		return "eight"
	case "9":
		return "nine"
	case "10":
		return "ten"
	case "J":
		return "jack"
	case "Q":
		return "queen"
	case "K":
		return "king"
	default:
		return "ace"
	}
}

func getCardColor(s string) string {
	if s == "H" || s == "D" {
		return "#ef4444" // Red
	}
	return "black"
}

// 4. RULES: Updated to switch on Short Codes ("A", "2", etc)
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

func getImageUrl(rank, suit string) string {
	// Convert Short Codes to Long Names for the URL
	fullSuit := getSuitName(suit) // "H" -> "hearts"
	fullRank := getRankName(rank) // "A" -> "ace"

	// Result: ".../hearts/ace-of-hearts.png"
	return fmt.Sprintf("https://outdrinkmeapi-dev.onrender.com/assets/images/cards/%s/%s-of-%s.png", fullSuit, fullRank, fullSuit)
}

// --- MAIN HANDLER ---

func (g *KingsCupLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	if !sender.IsHost {
		return
	}

	var payload struct {
		Action string `json:"action"`
		Type   string `json:"type"`
	}
	json.Unmarshal(msg, &payload)

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
			Suit:     getSuitName(drawn.Suit), // "hearts"
			Value:    drawn.Rank,              // "A" (Display as A, not ace)
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