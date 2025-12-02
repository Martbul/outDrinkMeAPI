package utils

import (
	"fmt"
	"math/rand"
	"time"
)
type Card struct {
	Suit string `json:"suit"`
	Rank string `json:"rank"`
}

func GenerateNewDeck() []Card {
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


func GetSuitName(s string) string {
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

func GetRankName(r string) string {
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

func GetCardColor(s string) string {
	if s == "H" || s == "D" {
		return "#ef4444"
	}
	return "black"
}

func GetImageUrl(rank, suit string) string {
	fullSuit := GetSuitName(suit)
	fullRank := GetRankName(rank)

	return fmt.Sprintf("https://martbul.com/assets/images/cards/%s/%s-of-%s.png", fullSuit, fullRank, fullSuit)
}
