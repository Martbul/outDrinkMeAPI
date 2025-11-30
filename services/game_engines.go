package services

import (
	"math/rand"
	"sync"
	"time"
)

type Card struct {
	Suit string `json:"suit"`
	Rank string `json:"rank"`
}

// Fixed: Correctly seeding the random generator
func generateNewDeck() []Card {
	suits := []string{"H", "D", "C", "S"}
	ranks := []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}

	deck := make([]Card, 0, 52)

	for _, s := range suits {
		for _, r := range ranks {
			deck = append(deck, Card{Suit: s, Rank: r})
		}
	}

	// Create a local random source to be thread-safe and random every time
	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)

	r.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	return deck
}

type KingsCupLogic struct {
	Deck        []Card     `json:"deck"`
	CurrentCard *Card      `json:"current_card"`
	TurnCount   int        `json:"turn_count"`
	mu          sync.Mutex // <--- ADD THIS (The Lock)
}

func (g *KingsCupLogic) InitState() interface{} {
	g.mu.Lock() // Lock just in case
	defer g.mu.Unlock()

	g.Deck = generateNewDeck()
	g.CurrentCard = nil
	g.TurnCount = 0

	return map[string]interface{}{
		"status":     "ready",
		"deck_count": len(g.Deck),
		"turn_count": 0,
	}
}

func (g *KingsCupLogic) GetRandomCard() *Card {
	g.mu.Lock()         // <--- LOCK: Only one person draws at a time
	defer g.mu.Unlock() // <--- UNLOCK: When function finishes

	if len(g.Deck) == 0 {
		return nil
	}

	randomCard := g.Deck[0]
	g.Deck = g.Deck[1:] // Modify the slice safely

	return &randomCard
}

func (g *KingsCupLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	// 1. Parse payload
	// var payload struct {
	// 	Action string `json:"action"`
	// }
	// if err := json.Unmarshal(msg, &payload); err != nil {
	// 	return
	// }

	// // 2. Switch
	// switch payload.Action {
	// case "draw_card":
	// 	// GetRandomCard handles the locking internally now
	// 	card := g.GetRandomCard()
		
	// 	if card == nil {
	// 		s.BroadcastJSON(map[string]string{"type": "game_over"})
	// 		return
	// 	}

	// 	// Update state safely
	// 	g.mu.Lock()
	// 	g.CurrentCard = card
	// 	g.TurnCount++
	// 	// Capture values to send (so we can unlock early)
	// 	turnCount := g.TurnCount
	// 	g.mu.Unlock()

	// 	// Broadcast result
	// 	s.BroadcastJSON(map[string]interface{}{
	// 		"type":       "card_drawn",
	// 		"player":     sender.ID, // Make sure Client struct has ID
	// 		"card":       card,
	// 		"turn_count": turnCount,
	// 	})
	// }
}

type BurnBookLogic struct{}

func (g *BurnBookLogic) InitState() interface{} {
	return nil
}

func (g *BurnBookLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	s.Broadcast <- msg
}
