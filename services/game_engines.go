package services

import (
	"math/rand"

	"time"
)

type Card struct {
	Suit string `json:"suit"`
	Rank string `json:"rank"`
}

func generateNewDeck() []Card {
	suits := []string{"H", "D", "C", "S"}
	ranks := []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}

	deck := make([]Card, 0, 52)

	for _, s := range suits {
		for _, r := range ranks {
			deck = append(deck, Card{Suit: s, Rank: r})
		}
	}

	// Shuffle
	rand.NewSource(time.Now().UnixNano()) 
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})

	return deck
}

func getRuleForCard(rank string) string {
	switch rank {
	case "A":
		return "Waterfall - Everyone drinks!"
	case "2":
		return "You - Pick someone to drink"
	case "3":
		return "Me - You drink"
	case "4":
		return "Floor - Last to touch floor drinks"
	case "5":
		return "Guys - All guys drink"
	case "6":
		return "Chicks - All girls drink"
	case "7":
		return "Heaven - Last to point up drinks"
	case "8":
		return "Mate - Pick a drinking buddy"
	case "9":
		return "Rhyme - Say a phrase, go around rhyming"
	case "10":
		return "Categories - Pick a category"
	case "J":
		return "Never Have I Ever"
	case "Q":
		return "Questions - Ask questions only"
	case "K":
		return "King's Cup - Pour into the center cup!"
	default:
		return "Drink"
	}
}

type KingsCupLogic struct {
	Deck        []Card `json:"deck"`         // The stack of cards
	CurrentCard *Card  `json:"current_card"` // The last card drawn
	TurnCount   int    `json:"turn_count"`
}

func (g *KingsCupLogic) InitState() interface{} {
	g.Deck = generateNewDeck()
	g.CurrentCard = nil
	g.TurnCount = 0

	return map[string]interface{}{
		"status":     "ready",
		"deck_count": len(g.Deck),
		"turn_count": 0,
	}
}

func (g *KingsCupLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	// Simple Echo for now, replace with real game logic later
	s.Broadcast <- msg

	// 1. Parse the incoming JSON
	// var payload struct {
	// 	Action string `json:"action"` // e.g., "draw_card"
	// }

	// if err := json.Unmarshal(msg, &payload); err != nil {
	// 	return // Ignore bad JSON
	// }

	// // 2. Switch on Action
	// switch payload.Action {
	// case "draw_card":
	// 	g.handleDrawCard(s, sender)
	// case "restart":
	// 	g.InitState()
	// 	s.BroadcastJSON(map[string]string{"type": "game_reset"})
	// }
}

type BurnBookLogic struct{}

func (g *BurnBookLogic) InitState() interface{} {
	return nil
}

func (g *BurnBookLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	s.Broadcast <- msg
}
