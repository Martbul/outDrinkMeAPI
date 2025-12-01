package services

import (
	"encoding/json"
	"fmt"
	"log"
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
	Phase          string        `json:"phase"`                  // "collecting", "voting", "results", "game_over"
	QuestionText   string        `json:"questionText,omitempty"` // Current question text
	CollectedCount int           `json:"collectedCount,omitempty"`
	
	// For Voting Phase
	TimeRemaining  int           `json:"timeRemaining,omitempty"` // Optional: tell UI how long they have
	CurrentNumber  int           `json:"currentNumber,omitempty"` // "Question 1 of 3"
	TotalQuestions int           `json:"totalQuestions,omitempty"`

	Players        []PlayerInfo  `json:"players,omitempty"`
	RoundResults   *RoundResult  `json:"roundResults,omitempty"`
	HasVoted       bool          `json:"hasVoted,omitempty"`
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
	
	// Logic State
	VotingIndex  int // Tracks which question we are voting on
	RevealIndex  int // Tracks which result we are showing
	
	Votes        map[int]map[string]int  // [QuestionIndex] -> [CandidateID] -> Count
	WhoVoted     map[int]map[string]bool // [QuestionIndex] -> [VoterID] -> bool
	
	// Timer handling
	Timer        *time.Timer
	mu           sync.Mutex
}


func (g *BurnBookLogic) InitState() interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Phase = "collecting"
	g.Questions = make([]string, 0)
	g.Votes = make(map[int]map[string]int)
	g.WhoVoted = make(map[int]map[string]bool)
	g.VotingIndex = 0
	g.RevealIndex = -1 // Starts at -1 so first "next" goes to 0

	return BurnBookGameState{
		Phase:          "collecting",
		CollectedCount: 0,
	}
}
func (g *BurnBookLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	var request struct {
		Type     string `json:"type"`
		Payload  string `json:"payload"`
		TargetID string `json:"targetId"`
	}

	if err := json.Unmarshal(msg, &request); err != nil {
		fmt.Println("JSON Error:", err)
		return
	}

	g.mu.Lock()
	// Note: We defer Unlock inside specific blocks or at the end, 
	// but because we use a Timer that needs the lock, we must be careful not to deadlock.
	// For this logic, we will lock primarily for state updates.
	defer g.mu.Unlock()

	log.Println("Received Action:", request.Type)

	// --- 1. SUBMIT QUESTION ---
	if request.Type == "submit_question" && g.Phase == "collecting" {
		if request.Payload == "" {
			return
		}
		g.Questions = append(g.Questions, request.Payload)

		broadcast(s, GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:          "collecting",
				CollectedCount: len(g.Questions),
			},
		})
		return
	}

	// --- 2. START VOTING (Host) ---
	if request.Type == "start_voting" && sender.IsHost && g.Phase == "collecting" {
		if len(g.Questions) == 0 {
			return
		}

		g.Phase = "voting"
		g.VotingIndex = 0
		g.Votes = make(map[int]map[string]int)
		g.WhoVoted = make(map[int]map[string]bool)

		// Unlock before calling the timer function to avoid deadlock
		g.mu.Unlock() 
		g.startQuestionTimer(s) // Start the automatic flow
		g.mu.Lock() // Relock for the defer Unlock
		return
	}

	// --- 3. CAST VOTE ---
	if request.Type == "vote_player" && g.Phase == "voting" {
		if request.TargetID == "" {
			return
		}
		
		// Setup maps
		if g.Votes[g.VotingIndex] == nil {
			g.Votes[g.VotingIndex] = make(map[string]int)
		}
		if g.WhoVoted[g.VotingIndex] == nil {
			g.WhoVoted[g.VotingIndex] = make(map[string]bool)
		}

		// Prevent double voting
		if g.WhoVoted[g.VotingIndex][sender.UserID] {
			return
		}

		g.Votes[g.VotingIndex][request.TargetID]++
		g.WhoVoted[g.VotingIndex][sender.UserID] = true

		// Check if everyone has voted to skip the timer?
		// For now, let's keep the timer running to keep it simple (30s fixed),
		// OR you can broadcast the updated "HasVoted" state.
		broadcastVotingState(s, g)
		return
	}

	// --- 4. REVEAL NEXT RESULT (Host Only) ---
	// This steps through the results one by one
	if request.Type == "next_reveal" && sender.IsHost && g.Phase == "results" {
		g.RevealIndex++

		// Check if we showed all questions
		if g.RevealIndex >= len(g.Questions) {
			g.Phase = "game_over"
			broadcast(s, GameStatePayload{
				Action: "game_update",
				GameState: BurnBookGameState{
					Phase: "game_over",
				},
			})
			return
		}

		// Calculate winner for the *current reveal index*
		winnerID, voteCount := g.calculateWinner(g.RevealIndex)

		broadcast(s, GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:        "results",
				QuestionText: g.Questions[g.RevealIndex],
				RoundResults: &RoundResult{
					WinnerID: winnerID,
					Votes:    voteCount,
				},
				Players: s.getPlayersList(),
			},
		})
		return
	}
}

// --- HELPER: AUTOMATIC QUESTION TIMER ---
// This function handles the 30s logic and recursively calls itself
func (g *BurnBookLogic) startQuestionTimer(s *Session) {
	g.mu.Lock()
	
	// Stop if game is over or phase changed unexpectedly
	if g.VotingIndex >= len(g.Questions) {
		g.Phase = "results"
		g.RevealIndex = -1 // Reset reveal index
		
		// Notify clients that voting is done, waiting for host to reveal
		broadcast(s, GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase: "results_wait", // UI should show "Waiting for Host to reveal..."
			},
		})
		g.mu.Unlock()
		return
	}

	// Broadcast Current Question
	broadcastVotingState(s, g)
	
	currentIndex := g.VotingIndex // Capture current index for closure
	g.mu.Unlock()

	// Wait 30 seconds
	// Note: In a production app, you might want to allow "early skip" if everyone voted.
	// We use AfterFunc so we don't block a thread, but here we just need to trigger the next step.
	go func() {
		time.Sleep(30 * time.Second)
		
		g.mu.Lock()
		// Check if we are still on the same question (prevents race conditions if game ended)
		if g.Phase == "voting" && g.VotingIndex == currentIndex {
			g.VotingIndex++ // Move to next
			g.mu.Unlock()
			g.startQuestionTimer(s) // Recursion for next question
		} else {
			g.mu.Unlock()
		}
	}()
}

func broadcast(s *Session, payload GameStatePayload) {
	bytes, _ := json.Marshal(payload)
	s.Broadcast <- bytes
}

func broadcastVotingState(s *Session, g *BurnBookLogic) {
	// Helper to send the current voting state to everyone
	for client := range s.Clients {
		hasVoted := false
		if g.WhoVoted[g.VotingIndex] != nil {
			hasVoted = g.WhoVoted[g.VotingIndex][client.UserID]
		}

		response := GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:          "voting",
				QuestionText:   g.Questions[g.VotingIndex],
				CurrentNumber:  g.VotingIndex + 1,
				TotalQuestions: len(g.Questions),
				Players:        s.getPlayersList(),
				HasVoted:       hasVoted,
				TimeRemaining:  30, // UI can start a countdown
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