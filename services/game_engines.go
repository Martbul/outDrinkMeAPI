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
	Action    string      `json:"action"` // "game_update"
	GameState interface{} `json:"gameState"`
}

type KingsCupGameState struct {
	CurrentCard    *ClientCard `json:"currentCard"`
	CardsRemaining int         `json:"cardsRemaining"`
	GameOver       bool        `json:"gameOver"`
}

type RoundResult struct {
	WinnerID string `json:"winnerId"`
	Votes    int    `json:"votes"`
}

type BurnBookGameState struct {
	Phase          string        `json:"phase"`                    
	QuestionText   string        `json:"questionText,omitempty"`   
	CollectedCount int           `json:"collectedCount,omitempty"`
	Players        []PlayerInfo  `json:"players,omitempty"`        
	RoundResults   *RoundResult  `json:"roundResults,omitempty"`   
	HasVoted       bool          `json:"hasVoted,omitempty"`
	MyVote         string        `json:"myVote,omitempty"`
}

type PlayerInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
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
	return KingsCupGameState{ // initial state
		CurrentCard:    nil,
		CardsRemaining: 52,
		GameOver:       false,
	}
}

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
		return "#ef4444"
	}
	return "black"
}

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
	fullSuit := getSuitName(suit)
	fullRank := getRankName(rank)

	return fmt.Sprintf("https://outdrinkmeapi-dev.onrender.com/assets/images/cards/%s/%s-of-%s.png", fullSuit, fullRank, fullSuit)
}

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
				GameState: KingsCupGameState{
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
			GameState: KingsCupGameState{
				CurrentCard:    &clientCard,
				CardsRemaining: remaining,
				GameOver:       false,
			},
		}

		bytes, _ := json.Marshal(response)
		s.Broadcast <- bytes
	}
}

// init
// host licks start game
// each client sends a question
// host clicks on "BURN" button
// reange to questions wuth each client as an andswer(20 sec timer) & host button for next question
// after all questuions are done den range throuh anwsers
// end game
type BurnBookLogic struct {
	Phase        string
	Questions    []string
	CurrentIndex int
	Votes        map[int]map[string]int
	WhoVoted     map[int]map[string]bool
	mu           sync.Mutex
}

func (g *BurnBookLogic) InitState() interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Phase = "collecting"
	g.Questions = make([]string, 0)
	g.Votes = make(map[int]map[string]int)
	g.WhoVoted = make(map[int]map[string]bool) 
	g.CurrentIndex = 0

	return BurnBookGameState{
		Phase:          "collecting",
		CollectedCount: 0,
	}
}

func (g *BurnBookLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	var request struct {
		Type    string                 `json:"type"`
		Payload map[string]interface{} `json:"payload"`
	}

	if err := json.Unmarshal(msg, &request); err != nil {
		fmt.Println("JSON Error:", err)
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// --- 1. SUBMIT QUESTION ---
	if request.Type == "submit_question" && g.Phase == "collecting" {
		// Ensure payload exists in the map
		if request.Payload == nil {
			return
		}
		qText, ok := request.Payload["payload"].(string)
		if !ok || qText == "" {
			return
		}

		g.Questions = append(g.Questions, qText)

		response := GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:          "collecting",
				CollectedCount: len(g.Questions),
			},
		}
		broadcast(s, response)
		return
	}

	// --- 2. START VOTING ---
	if request.Type == "start_voting" && sender.IsHost && g.Phase == "collecting" {
		if len(g.Questions) == 0 {
			return
		}

		g.Phase = "voting"
		g.CurrentIndex = 0
		g.Votes = make(map[int]map[string]int)
		g.WhoVoted = make(map[int]map[string]bool)

		broadcastVotingState(s, g)
		return
	}

	// --- 3. CAST VOTE ---
	if request.Type == "vote_player" && g.Phase == "voting" {
		if request.Payload == nil {
			return
		}
		targetID, _ := request.Payload["targetId"].(string)
		if targetID == "" {
			return
		}

		// Initialize maps if nil
		if g.Votes[g.CurrentIndex] == nil {
			g.Votes[g.CurrentIndex] = make(map[string]int)
		}
		if g.WhoVoted[g.CurrentIndex] == nil {
			g.WhoVoted[g.CurrentIndex] = make(map[string]bool)
		}

		// Check if Sender already voted
		if g.WhoVoted[g.CurrentIndex][sender.UserID] {
			return
		}

		// Record Vote
		g.Votes[g.CurrentIndex][targetID]++
		g.WhoVoted[g.CurrentIndex][sender.UserID] = true

		broadcastVotingState(s, g)
		return
	}

	// --- 4. REVEAL RESULTS ---
	if request.Type == "reveal_results" && sender.IsHost && g.Phase == "voting" {
		g.Phase = "results"

		winnerID, voteCount := g.calculateWinner(g.CurrentIndex)

		response := GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:        "results",
				QuestionText: g.Questions[g.CurrentIndex],
				// Now valid because we added RoundResult to the struct
				RoundResults: &RoundResult{ 
					WinnerID: winnerID,
					Votes:    voteCount,
				},
				// Now valid because we added Players to the struct
				Players: s.getPlayersList(), 
			},
		}
		broadcast(s, response)
		return
	}

	// --- 5. NEXT QUESTION ---
	if request.Type == "next_question" && sender.IsHost && g.Phase == "results" {
		g.CurrentIndex++

		// Check if game is over
		if g.CurrentIndex >= len(g.Questions) {
			response := GameStatePayload{
				Action: "game_update",
				GameState: BurnBookGameState{
					Phase: "game_over",
				},
			}
			broadcast(s, response)
			return
		}

		// Start voting for next question
		g.Phase = "voting"
		broadcastVotingState(s, g)
		return
	}
}

func broadcast(s *Session, payload GameStatePayload) {
	bytes, _ := json.Marshal(payload)
	s.Broadcast <- bytes
}

func broadcastVotingState(s *Session, g *BurnBookLogic) {
	for client := range s.Clients {

		hasVoted := false
		if g.WhoVoted[g.CurrentIndex] != nil {
			hasVoted = g.WhoVoted[g.CurrentIndex][client.UserID]
		}

		response := GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:        "voting",
				QuestionText: g.Questions[g.CurrentIndex],
				Players:      s.getPlayersList(), 
				HasVoted:     hasVoted,           
			},
		}

		bytes, _ := json.Marshal(response)
		client.Send <- bytes
	}
}

func (g *BurnBookLogic) calculateWinner(idx int) (string, int) {
	votes := g.Votes[idx]
	if len(votes) == 0 {
		return "", 0
	}

	winnerID := ""
	maxVotes := -1

	for id, count := range votes {
		if count > maxVotes {
			maxVotes = count
			winnerID = id
		}
	}
	return winnerID, maxVotes
}