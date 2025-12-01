package services

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"
)

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
	Action    string      `json:"action"`
	GameState interface{} `json:"gameState"`
}

type KingsCupGameState struct {
	CurrentCard    *ClientCard `json:"currentCard"`
	CardsRemaining int         `json:"cardsRemaining"`
	GameOver       bool        `json:"gameOver"`
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

type BurnBookLogic struct {
	mu        sync.Mutex
	Timer     *time.Timer
	Questions []string
	Phase string
	Votes    map[int]map[string]int  // [QuestionIndex] -> [CandidateID] -> Count
	WhoVoted map[int]map[string]bool // [QuestionIndex] -> [VoterID] -> bool
	VotingIndex int
	RevealIndex int

}

// type RoundResult struct {
// 	WinnerID string `json:"winnerId"`
// 	Votes    int    `json:"votes"`
// }

type RoundResult struct {
	WinnerID string            `json:"winnerId"` // Helper to identify the top victim easily
	Results  []PlayerRoundInfo `json:"results"`  // List of all players who got votes
}

type PlayerRoundInfo struct {
	UserID string `json:"userId"`
	Votes  int    `json:"votes"`
}


type BurnBookGameState struct {
	Phase          string `json:"phase"`
	QuestionText   string `json:"questionText,omitempty"` // Current question text
	CollectedCount int    `json:"collectedCount,omitempty"`

	TimeRemaining  int `json:"timeRemaining,omitempty"` // Optional: tell UI how long they have
	CurrentNumber  int `json:"currentNumber,omitempty"` // "Question 1 of 3"
	TotalQuestions int `json:"totalQuestions,omitempty"`

	Players      []PlayerInfo `json:"players,omitempty"`
	RoundResults *RoundResult `json:"roundResults,omitempty"`
	HasVoted     bool         `json:"hasVoted,omitempty"`
}

type PlayerInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

func (g *BurnBookLogic) InitState() interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Phase = "collecting"
	g.Questions = make([]string, 0)
	g.Votes = make(map[int]map[string]int)
	g.WhoVoted = make(map[int]map[string]bool)
	g.VotingIndex = 0
	g.RevealIndex = -1

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
		g.mu.Lock()             // Relock for the defer Unlock
		return
	}

	if request.Type == "vote_player" && g.Phase == "voting" {
		if request.TargetID == "" {
			return
		}

		if g.Votes[g.VotingIndex] == nil {
			g.Votes[g.VotingIndex] = make(map[string]int)
		}
		if g.WhoVoted[g.VotingIndex] == nil {
			g.WhoVoted[g.VotingIndex] = make(map[string]bool)
		}

		if g.WhoVoted[g.VotingIndex][sender.UserID] {
			return
		}

		g.Votes[g.VotingIndex][request.TargetID]++
		g.WhoVoted[g.VotingIndex][sender.UserID] = true

		//! Check if everyone has voted to skip the timer?
		// For now, let's keep the timer running to keep it simple (30s fixed),
		// OR you can broadcast the updated "HasVoted" state.
		broadcastVotingState(s, g)
		return
	}

	if request.Type == "next_reveal" && sender.IsHost && g.Phase == "results" {
		g.RevealIndex++

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

		roundResults := g.getRoundResults(g.RevealIndex)

			broadcast(s, GameStatePayload{
			Action: "game_update",
			GameState: BurnBookGameState{
				Phase:        "results",
				QuestionText: g.Questions[g.RevealIndex],
				RoundResults: roundResults, // Sending the full struct
				Players:      s.getPlayersList(),
			},
		})
		return
	}
}


func (g *BurnBookLogic) getRoundResults(idx int) *RoundResult {
	votesMap := g.Votes[idx]
	
	results := make([]PlayerRoundInfo, 0)
	winnerID := ""
	maxVotes := -1

	// 1. Convert Map to Slice
	for userID, count := range votesMap {
		results = append(results, PlayerRoundInfo{
			UserID: userID,
			Votes:  count,
		})

		// Track winner
		if count > maxVotes {
			maxVotes = count
			winnerID = userID
		} else if count == maxVotes {
			// Handle ties if necessary (here we just keep the first one found or random map order)
			// You could leave winnerID as is
		}
	}

	// 2. Sort the slice by Votes (Descending) so the Client receives an ordered list
	sort.Slice(results, func(i, j int) bool {
		return results[i].Votes > results[j].Votes
	})

	return &RoundResult{
		WinnerID: winnerID,
		Results:  results,
	}
}

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

type MafiaLogic struct {
	// 1. Mutex is usually placed first.
	// This is good for cache locality and indicates it protects the fields below.
	mu sync.Mutex

	// 2. Larger "Structure" types (Strings are 16 bytes: pointer + length)
	Phase       string
	MafiaTarget string

	// 3. Pointers and Maps (8 bytes each)
	timer   *time.Timer
	Roles   map[string]string
	IsAlive map[string]bool
	Votes   map[string]string
}

type MafiaGameState struct {
	// 1. Slices (24 bytes each)
	AlivePlayers []PlayerInfo `json:"alivePlayers"`
	DeadPlayers  []PlayerInfo `json:"deadPlayers"`

	// 2. Strings (16 bytes each)
	Phase   string `json:"phase"`
	Message string `json:"message"`
	MyRole  string `json:"myRole,omitempty"`
	Winner  string `json:"winner,omitempty"`

	// 3. Ints (8 bytes on 64-bit)
	TimeLeft int `json:"timeLeft"`
}

func (g *MafiaLogic) InitState() interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Phase = "LOBBY"
	g.Roles = make(map[string]string)
	g.IsAlive = make(map[string]bool)
	g.Votes = make(map[string]string)

	return MafiaGameState{
		Phase:   "LOBBY",
		Message: "Waiting for Host to start...",
	}
}

func (g *MafiaLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	var payload struct {
		Type     string `json:"type"`
		TargetID string `json:"targetId"`
	}

	if err := json.Unmarshal(msg, &payload); err != nil {
		log.Println("Mafia JSON Error:", err)
		return
	}

	log.Println(payload.Type)

	g.mu.Lock()
	// Note: We intentionally DO NOT defer Unlock() here globally
	// because some paths (like timers) need to unlock early to avoid deadlocks
	// or need to hold locks across helper calls. We will handle unlocking manually.

	if payload.Type == "start_game" && sender.IsHost && g.Phase == "LOBBY" {
		if len(s.Clients) < 3 {
			// Send error/warning to host? For now just ignore or log
			g.mu.Unlock()
			return
		}

		g.assignRoles(s) // Helper to shuffle and assign
		// g.IsAlive = make(map[string]bool)
		for client := range s.Clients {
			g.IsAlive[client.UserID] = true
		}

		g.mu.Unlock() // Unlock before starting the timer chain
		g.startNightPhase(s)
		return
	}

	if payload.Type == "kill" && g.Phase == "NIGHT" {
		if g.Roles[sender.UserID] == "MAFIA" && g.IsAlive[payload.TargetID] {
			g.MafiaTarget = payload.TargetID
			// Ack to Mafia only could happen here, but we'll wait for phase end
		}
		g.mu.Unlock()
		return
	}

	if payload.Type == "vote" && g.Phase == "VOTING" {
		if g.IsAlive[sender.UserID] && g.IsAlive[payload.TargetID] {
			g.Votes[sender.UserID] = payload.TargetID

			// Optional: Broadcast that X has voted (without revealing who)
			// to keep pressure up
		}
		g.mu.Unlock()
		return
	}

	g.mu.Unlock()

}

func (g *MafiaLogic) startNightPhase(s *Session) {
	g.mu.Lock()
	g.Phase = "NIGHT"
	g.MafiaTarget = "" // Reset target
	duration := 30     // 30 seconds for Mafia to decide

	// Notify Everyone
	g.broadcastState(s, "NIGHT", "Night has fallen. Sleep...", duration)

	// Send PRIVATE message to Mafia
	for client := range s.Clients {
		if g.Roles[client.UserID] == "MAFIA" {
			msg := map[string]interface{}{
				"action":  "system_message",
				"content": "You are the Mafia. Select a target to Kill!",
			}
			data, _ := json.Marshal(msg)
			client.Send <- data
		}
	}
	g.mu.Unlock()

	// Timer
	go func() {
		time.Sleep(time.Duration(duration) * time.Second)
		g.resolveNight(s)
	}()
}

func (g *MafiaLogic) resolveNight(s *Session) {
	g.mu.Lock()

	deathMessage := "The night was quiet."

	// Check if Mafia selected a target
	if g.MafiaTarget != "" {
		g.IsAlive[g.MafiaTarget] = false
		// Find username for message
		victimName := "Unknown"
		for client := range s.Clients {
			if client.UserID == g.MafiaTarget {
				victimName = client.Username
				break
			}
		}
		deathMessage = fmt.Sprintf("Sadly, %s was killed in the night!", victimName)
	} else {
		deathMessage = "The Mafia failed to act! They must take a penalty SHOT!"
	}

	// Check Game Over (Mafia Wins if they equal Civilians)
	if g.checkWinCondition(s) {
		g.mu.Unlock()
		return
	}

	g.mu.Unlock()
	g.startDayPhase(s, deathMessage)
}

func (g *MafiaLogic) startDayPhase(s *Session, morningMsg string) {
	g.mu.Lock()
	g.Phase = "DAY"
	duration := 360 // 360 seconds discussion
	g.broadcastState(s, "DAY", morningMsg+" Discuss who is guilty!", duration)
	g.mu.Unlock()

	go func() {
		time.Sleep(time.Duration(duration) * time.Second)
		g.startVotingPhase(s)
	}()
}

func (g *MafiaLogic) startVotingPhase(s *Session) {
	g.mu.Lock()
	g.Phase = "VOTING"
	g.Votes = make(map[string]string) // Reset votes
	duration := 30
	g.broadcastState(s, "VOTING", "Vote for execution! If there is a tie, everyone drinks.", duration)
	g.mu.Unlock()

	go func() {
		time.Sleep(time.Duration(duration) * time.Second)
		g.resolveVoting(s)
	}()
}

func (g *MafiaLogic) resolveVoting(s *Session) {
	g.mu.Lock()

	// Tally Votes
	voteCounts := make(map[string]int)
	for _, target := range g.Votes {
		voteCounts[target]++
	}

	// Find max
	maxVotes := 0
	victimID := ""
	isTie := false

	for target, count := range voteCounts {
		if count > maxVotes {
			maxVotes = count
			victimID = target
			isTie = false
		} else if count == maxVotes {
			isTie = true
		}
	}

	resultMsg := ""

	if isTie || maxVotes == 0 {
		resultMsg = "Vote was a tie (or empty). No one dies. EVERYONE DRINKS!"
	} else {
		g.IsAlive[victimID] = false
		// Get Name
		victimName := ""
		for client := range s.Clients {
			if client.UserID == victimID {
				victimName = client.Username
				break
			}
		}
		resultMsg = fmt.Sprintf("The town has decided. %s was executed.", victimName)
	}

	if g.checkWinCondition(s) {
		g.mu.Unlock()
		return
	}

	g.mu.Unlock()

	// Loop back to Night
	// Small delay to read results
	go func() {
		// Send a quick result update before night
		payload := GameStatePayload{
			Action: "game_update",
			GameState: MafiaGameState{
				Phase:   "RESULTS",
				Message: resultMsg,
			},
		}
		bytes, _ := json.Marshal(payload)
		s.Broadcast <- bytes

		time.Sleep(5 * time.Second)
		g.startNightPhase(s)
	}()
}

func (g *MafiaLogic) assignRoles(s *Session) {
	// Simple Logic: 1 Mafia, rest Civilians
	// For 7+ players, maybe 2 Mafia, but let's keep it simple

	ids := make([]string, 0, len(s.Clients))
	for client := range s.Clients {
		ids = append(ids, client.UserID)
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })

	// Assign Logic
	for i, id := range ids {
		role := "CIVILIAN"
		if i == 0 { // First shuffled ID is Mafia
			role = "MAFIA"
		}
		g.Roles[id] = role
	}

	// Send Private Role Info to Clients immediately
	for client := range s.Clients {
		role := g.Roles[client.UserID]
		payload := GameStatePayload{
			Action: "game_update",
			GameState: MafiaGameState{
				Phase:  "LOBBY",
				MyRole: role, // UI should trigger a popup "You are X"
			},
		}
		bytes, _ := json.Marshal(payload)
		client.Send <- bytes
	}
}

func (g *MafiaLogic) checkWinCondition(s *Session) bool {
	mafiaCount := 0
	civCount := 0

	for id, alive := range g.IsAlive {
		if alive {
			if g.Roles[id] == "MAFIA" {
				mafiaCount++
			} else {
				civCount++
			}
		}
	}

	winner := ""
	if mafiaCount == 0 {
		winner = "CIVILIANS"
	} else if mafiaCount >= civCount {
		winner = "MAFIA"
	}

	if winner != "" {
		g.Phase = "GAME_OVER"
		g.broadcastState(s, "GAME_OVER", fmt.Sprintf("GAME OVER! %s WIN! Losers finish their drinks.", winner), 0)
		return true
	}
	return false
}

func (g *MafiaLogic) broadcastState(s *Session, phase string, msg string, timeSec int) {
	// Helper to gather player lists
	alive := []PlayerInfo{}
	dead := []PlayerInfo{}

	for client := range s.Clients {
		p := PlayerInfo{ID: client.UserID, Username: client.Username}
		if g.IsAlive[client.UserID] {
			alive = append(alive, p)
		} else {
			dead = append(dead, p)
		}
	}

	payload := GameStatePayload{
		Action: "game_update",
		GameState: MafiaGameState{
			Phase:        phase,
			Message:      msg,
			TimeLeft:     timeSec,
			AlivePlayers: alive,
			DeadPlayers:  dead,
		},
	}

	bytes, _ := json.Marshal(payload)
	s.Broadcast <- bytes
}
