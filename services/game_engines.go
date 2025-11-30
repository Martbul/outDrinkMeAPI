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

type BurnBookGameState struct {
	Phase           string            `json:"phase"`           // "collecting", "voting", "results"
	QuestionText    string            `json:"questionText"`    // Current question text
	CollectedCount  int               `json:"collectedCount"`  // How many questions collected so far
	Voters          []PlayerInfo      `json:"voters"`          // List of players to vote for (candidates)
	Winner          string            `json:"winner,omitempty"`// Who won the vote (for results phase)
	Votes           map[string]int    `json:"votes,omitempty"` // Vote distribution (for results phase)
}
type PlayerInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
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
	Phase           string                  
	Questions       []string                  
	CurrentIndex    int                       
	Votes           map[int]map[string]int    
	mu              sync.Mutex
}

func (g *BurnBookLogic) InitState() interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	g.Phase = "collecting"
	g.Questions = make([]string, 0)
	g.Votes = make(map[int]map[string]int)
	g.CurrentIndex = 0

	return BurnBookGameState{
		Phase:          "collecting",
		CollectedCount: 0,
	}
}

func (g *BurnBookLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	var payload struct {
		Type    string `json:"type"`
		Payload string `json:"payload"` // For submitting question ("Who is ugly?") or voting ("userID")
	}
	json.Unmarshal(msg, &payload)

	g.mu.Lock()
	defer g.mu.Unlock()

	// --- 1. SUBMIT QUESTION (Anyone, Collecting Phase) ---
	if payload.Type == "submit_question" && g.Phase == "collecting" {
		g.Questions = append(g.Questions, payload.Payload)
		
		// Broadcast update so everyone sees count go up
		response := GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:          "collecting",
				CollectedCount: len(g.Questions),
			},
		}
		bytes, _ := json.Marshal(response)
		s.Broadcast <- bytes
		return
	}

	// --- 2. START VOTING / BURN (Host Only) ---
	if payload.Type == "start_voting" && sender.IsHost && g.Phase == "collecting" {
		if len(g.Questions) == 0 {
			return // Cannot start without questions
		}
		
		g.Phase = "voting"
		g.CurrentIndex = 0
		
		// Helper to get all players in session for the UI buttons
		candidates := s.getPlayersList()

		response := GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:        "voting",
				QuestionText: g.Questions[0],
				Voters:       candidates,
			},
		}
		bytes, _ := json.Marshal(response)
		s.Broadcast <- bytes
		return
	}

	// --- 3. CAST VOTE (Anyone, Voting Phase) ---
	if payload.Type == "submit_vote" && g.Phase == "voting" {
		candidateID := payload.Payload
		
		// Initialize map if needed
		if g.Votes[g.CurrentIndex] == nil {
			g.Votes[g.CurrentIndex] = make(map[string]int)
		}
		
		g.Votes[g.CurrentIndex][candidateID]++
		// Note: In a real app, you should check if sender already voted to prevent spam
		return
	}

	// --- 4. NEXT (Host Only: Next Question OR Go To Results) ---
	if payload.Type == "next_step" && sender.IsHost {
		
		if g.Phase == "voting" {
			g.CurrentIndex++
			
			// If we ran out of questions, switch to RESULTS phase
			if g.CurrentIndex >= len(g.Questions) {
				g.Phase = "results"
				g.CurrentIndex = 0 // Reset to show first result
				
				winner, counts := g.calculateWinner(g.CurrentIndex)
				
				response := GameStatePayload{
					Action: "game_update",
					GameState: BurnBookGameState{
						Phase:        "results",
						QuestionText: g.Questions[g.CurrentIndex],
						Winner:       winner,
						Votes:        counts,
					},
				}
				bytes, _ := json.Marshal(response)
				s.Broadcast <- bytes
				return
			}
			
			// Otherwise, show next question for voting
			candidates := s.getPlayersList()
			response := GameStatePayload{
				Action: "game_update",
				GameState: BurnBookGameState{
					Phase:        "voting",
					QuestionText: g.Questions[g.CurrentIndex],
					Voters:       candidates,
				},
			}
			bytes, _ := json.Marshal(response)
			s.Broadcast <- bytes
			return
		}

		// --- 5. NEXT RESULT (Host Only, Results Phase) ---
		if g.Phase == "results" {
			g.CurrentIndex++
			
			// End of game check
			if g.CurrentIndex >= len(g.Questions) {
				// Reset or End Game logic here
				return 
			}

			winner, counts := g.calculateWinner(g.CurrentIndex)

			response := GameStatePayload{
				Action: "game_update",
				GameState: BurnBookGameState{
					Phase:        "results",
					QuestionText: g.Questions[g.CurrentIndex],
					Winner:       winner,
					Votes:        counts,
				},
			}
			bytes, _ := json.Marshal(response)
			s.Broadcast <- bytes
			return
		}
	}
}

func (g *BurnBookLogic) calculateWinner(questionIdx int) (string, map[string]int) {
	votes := g.Votes[questionIdx]
	maxVotes := -1
	winnerID := "No Votes"
	
	// Determine ID with max votes
	for id, count := range votes {
		if count > maxVotes {
			maxVotes = count
			winnerID = id
		}
	}
	
	// In a real app, you might want to look up the Username from the Session here,
	// but sending the ID back is fine if the Client can map ID -> Username
	return winnerID, votes
}