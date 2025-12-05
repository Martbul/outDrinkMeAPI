package services

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"outDrinkMeAPI/utils"
	"sort"
	"strings"
	"sync"
	"time"
)

type PlayerInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type GameStatePayload struct {
	Action    string      `json:"action"`
	GameState interface{} `json:"gameState"`
}

type ClientCard struct {
	Suit     string `json:"suit"`
	Value    string `json:"value"`
	Rule     string `json:"rule"`
	Color    string `json:"color"`
	ImageUrl string `json:"imageUrl"`
}

type KingsCupGameState struct {
	Players             []PlayerInfo            `json:"players,omitempty"`
	CustomRules         map[string][]string     `json:"customRules,omitempty"` // CHANGED: Now a slice of strings
	Buddies             map[string][]PlayerInfo `json:"buddies,omitempty"`     // PlayerID -> list of their buddies
	CurrentCard         *ClientCard             `json:"currentCard"`
	CardsRemaining      int                     `json:"cardsRemaining"`
	GameOver            bool                    `json:"gameOver"`
	CurrentPlayerTurnID *string                 `json:"currentPlayerTurnID,omitempty"` // ID of the player whose turn it is
	KingsInCup          int                     `json:"kingsInCup"`                    // To track how many kings have been drawn
	KingCupDrinker      *PlayerInfo             `json:"kingCupDrinker,omitempty"`      // The player who drew the last king
	GameStarted         bool                    `json:"gameStarted"`                   // Indicates if the game has officially started
}

type KingsCupLogic struct {
	mu              sync.Mutex
	Deck            []utils.Card
	CurrentCard     *utils.Card
	Timer           *time.Timer             // Unused in this logic, but kept for consistency if you need it later
	DrawingIndex    int                     // Index in the Players slice indicating whose turn it is
	Players         []PlayerInfo            // List of all players in the game (managed by Session, but stored here for game logic)
	Buddies         map[string][]PlayerInfo // Tracks who is buddies with whom (playerID -> []PlayerInfo)
	CustomRules     map[string][]string     // CHANGED: Now a slice of strings
	KingsDrawn      int                     // Tracks how many kings have been drawn
	LastKingDrinker string                  // Stores the ID of the player who drew the last king
	GameStarted     bool
}


func (g *KingsCupLogic) InitState(s *Session) interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Deck = utils.GenerateNewDeck()
	rand.Shuffle(len(g.Deck), func(i, j int) {
		g.Deck[i], g.Deck[j] = g.Deck[j], g.Deck[i]
	}) // Shuffle the deck! Important for games.
	g.CurrentCard = nil
	g.DrawingIndex = 0
	g.Buddies = make(map[string][]PlayerInfo)
	g.CustomRules = make(map[string][]string)
	g.KingsDrawn = 0
	g.LastKingDrinker = ""
	g.GameStarted = true

	// IMPORTANT: Populate g.Players from the session's clients
	g.Players = make([]PlayerInfo, 0, len(s.Clients))
	for client := range s.Clients {
		if client.UserID != "" {
			g.Players = append(g.Players, PlayerInfo{ID: client.UserID, Username: client.Username})
		}
	}
	// Ensure players are in a consistent order, e.g., by ID or username
	sort.Slice(g.Players, func(i, j int) bool {
		return g.Players[i].ID < g.Players[j].ID
	})

	var initialPlayerTurnID *string
	if len(g.Players) > 0 {
		initialPlayerTurnID = &g.Players[g.DrawingIndex].ID
	}

	initialState := KingsCupGameState{
		Players:             g.Players,
		CustomRules:         g.CustomRules,
		Buddies:             g.Buddies,
		CurrentCard:         nil,
		CardsRemaining:      len(g.Deck),
		GameOver:            false,
		CurrentPlayerTurnID: initialPlayerTurnID, // Set initial turn here
		KingsInCup:          0,
		KingCupDrinker:      nil,
		GameStarted:         true,
	}

	// Broadcast the initial game state immediately after initialization
	// This is crucial for all clients to get the correct starting state, including player list and first turn
	g.broadcastGameState(s) // Removed 'nil'

	return initialState
}
func (g *KingsCupLogic) GetGameStarted() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.GameStarted
}

func (g *KingsCupLogic) GetCurrentCard() *utils.Card {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.CurrentCard
}

func (g *KingsCupLogic) UpdatePlayers(currentClients map[*Client]bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	newPlayers := make([]PlayerInfo, 0, len(currentClients))
	currentPlayersMap := make(map[string]bool) // To quickly check existing players

	for client := range currentClients {
		if client.UserID != "" {
			playerInfo := PlayerInfo{ID: client.UserID, Username: client.Username}
			newPlayers = append(newPlayers, playerInfo)
			currentPlayersMap[client.UserID] = true
		}
	}

	// Remove buddies/rules for players who left
	for playerID := range g.Buddies {
		if _, exists := currentPlayersMap[playerID]; !exists {
			delete(g.Buddies, playerID)
			// Also remove this player from other players' buddy lists
			for otherPlayerID, buddies := range g.Buddies {
				for i, buddy := range buddies {
					if buddy.ID == playerID {
						g.Buddies[otherPlayerID] = append(g.Buddies[otherPlayerID][:i], g.Buddies[otherPlayerID][i+1:]...)
						break
					}
				}
			}
		}
	}
	for playerID := range g.CustomRules {
		if _, exists := currentPlayersMap[playerID]; !exists {
			delete(g.CustomRules, playerID)
		}
	}

	// If the current player's turn is no longer valid (player left), reset the index.
	if g.DrawingIndex >= len(newPlayers) && len(newPlayers) > 0 {
		g.DrawingIndex = 0
	} else if len(newPlayers) == 0 {
		g.DrawingIndex = 0 // Reset if no players left
	}

	g.Players = newPlayers
	log.Printf("KingsCupLogic Players updated. Current players: %v", g.Players)
	// After updating players, broadcast the comprehensive game state
	g.broadcastGameState(nil) // Removed 'nil' (Wait, your broadcastGameState takes session.
	// If called from UpdatePlayers where session might be tricky,
	// ensure you handle the session parameter correctly or pass it through).
}


func (g *KingsCupLogic) broadcastGameState(session *Session) {
	var currentPlayerTurnID *string
	if len(g.Players) > 0 {
		currentPlayerTurnID = &g.Players[g.DrawingIndex].ID
	}

	kingCupDrinkerInfo := g.GetPlayerInfoByID(g.LastKingDrinker)

	// LOGIC CHANGE: Generate ClientCard from g.CurrentCard state
	var clientCard *ClientCard
	if g.CurrentCard != nil {
		c := ClientCard{
			Suit:     utils.GetSuitName(g.CurrentCard.Suit),
			Value:    g.CurrentCard.Rank,
			Rule:     g.getRule(g.CurrentCard.Rank),
			Color:    utils.GetCardColor(g.CurrentCard.Suit),
			ImageUrl: utils.GetImageUrl(g.CurrentCard.Rank, g.CurrentCard.Suit),
		}
		clientCard = &c
	}

	state := KingsCupGameState{
		Players:             g.Players,
		CustomRules:         g.CustomRules,
		Buddies:             g.Buddies,
		CurrentCard:         clientCard, // Use the variable derived above
		CardsRemaining:      len(g.Deck),
		GameOver:            len(g.Deck) == 0,
		CurrentPlayerTurnID: currentPlayerTurnID,
		KingsInCup:          g.KingsDrawn,
		KingCupDrinker:      kingCupDrinkerInfo,
		GameStarted:         g.GameStarted,
	}

	response := GameStatePayload{
		Action:    "game_update",
		GameState: state,
	}

	bytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshalling game state: %v", err)
		return
	}

	// Send to the session's broadcast channel
	session.Broadcast <- bytes
}

// GetPlayerInfoByID is a helper to get player info from ID
func (g *KingsCupLogic) GetPlayerInfoByID(playerID string) *PlayerInfo {
	if playerID == "" {
		return nil
	}
	for _, p := range g.Players {
		if p.ID == playerID {
			return &p
		}
	}
	return nil
}

func (g *KingsCupLogic) getRule(rank string) string {
	switch rank {
	case "A":
		return "Waterfall - Start drinking at the same time as the person to your left. Don't stop until they do."
	case "2":
		return "You - Choose someone to drink"
	case "3":
		return "Me - You drink"
	case "4":
		return "Floor - The last person to touch the floor drinks"
	case "5":
		return "Guys - All Guys drink"
	case "6":
		return "Chicks - All girls drink"
	case "7":
		return "Heaven - Raise your hand to heaven. The last person to do so drinks"
	case "8":
		return "Mate - Choose a drinking buddy. Any time you drink, they drink"
	case "9":
		return "Rhyme - Say a word. The person to your right says a word that rhymes. The first person to fail drinks"
	case "10":
		return "Categories - Choose a category of things. The person to your right names something in that category. The first person to fail drinks"
	case "J":
		return "Never Have I Ever - Play never have i ever"
	case "Q":
		return "Question - Ask someone a question. That person then asks someone else a question. The first person to fail drinks"
	case "K":
		return "Kinkg's cup - Set a rule and pour some of your drink into the king's cup. Whoever draws the final king must drink the entire king's cup"
	default:
		return "Drink!"
	}
}

func (g *KingsCupLogic) HandleMessage(s *Session, sender *Client, msg []byte) {
	var request struct {
		Type           string  `json:"type"`
		ChosenBuddieID *string `json:"chosen_buddie_id,omitempty"`
		NewRule        string  `json:"new_rule,omitempty"`
	}

	if err := json.Unmarshal(msg, &request); err != nil {
		fmt.Println("JSON Error:", err)
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if len(g.Players) == 0 {
		log.Println("No players in the game logic. Cannot handle messages.")
		return
	}

	// Only the current player can draw a card or perform actions related to their turn
	// This check is crucial for turn-based games
	if g.DrawingIndex >= len(g.Players) || sender.UserID != g.Players[g.DrawingIndex].ID {
		if len(g.Players) > 0 {
			log.Printf("It's not %s's turn. Current turn is %s (%s) but %s (%s) tried to act.\n",
				sender.Username, g.Players[g.DrawingIndex].Username, g.Players[g.DrawingIndex].ID, sender.Username, sender.UserID)
		} else {
			log.Printf("It's not %s's turn. No players in game logic.\n", sender.Username)
		}
		// Potentially send an error back to the sender if needed for UI feedback
		return
	}

	switch request.Type {

	case "draw_card":
		if len(g.Deck) == 0 {
			// Game is over
			response := GameStatePayload{
				Action: "game_update",
				GameState: KingsCupGameState{
					CurrentCard:    nil,
					CardsRemaining: 0,
					GameOver:       true,
					KingsInCup:     g.KingsDrawn,
					KingCupDrinker: g.GetPlayerInfoByID(g.LastKingDrinker),
				},
			}
			bytes, _ := json.Marshal(response)
			s.Broadcast <- bytes
			return
		}

		drawn := g.Deck[0]
		g.Deck = g.Deck[1:]
		g.CurrentCard = &drawn // This updates the state

		// Check for 4th King logic
		if drawn.Rank == "K" {
			g.KingsDrawn++
			if g.KingsDrawn == 4 {
				g.LastKingDrinker = sender.UserID
				log.Printf("The 4th King has been drawn! %s must drink the King's Cup!\n", sender.Username)
			}
		}

		// LOGIC CHANGE: Just call broadcast. It will read g.CurrentCard automatically.
		g.broadcastGameState(s)

		if drawn.Rank == "8" {
			log.Printf("%s drew an 8. Waiting for buddy selection.\n", sender.Username)
			return
		} else if drawn.Rank == "K" {
			log.Printf("%s drew a King. Waiting for rule setting.\n", sender.Username)
			return
		}

		// Advance turn
		g.DrawingIndex = (g.DrawingIndex + 1) % len(g.Players)
		log.Printf("Turn advanced to %s (%s)\n", g.Players[g.DrawingIndex].Username, g.Players[g.DrawingIndex].ID)

		// LOGIC CHANGE: Broadcast update (Turn change).
		// The card will still be visible because g.CurrentCard is still set.
		g.broadcastGameState(s)

	case "choose_buddy":
		if g.CurrentCard == nil || g.CurrentCard.Rank != "8" {
			log.Printf("Cannot choose a buddy, an 8 was not just drawn by %s, or no card is drawn.\n", sender.Username)
			return
		}
		if request.ChosenBuddieID == nil || *request.ChosenBuddieID == "" {
			log.Println("No buddy chosen or invalid ID provided.")
			return
		}

		chosenBuddyInfo := g.GetPlayerInfoByID(*request.ChosenBuddieID)
		if chosenBuddyInfo == nil {
			log.Printf("Chosen buddy with ID %s not found.\n", *request.ChosenBuddieID)
			return
		}

		// Add buddy relationship (bidirectional)
		g.Buddies[sender.UserID] = append(g.Buddies[sender.UserID], *chosenBuddyInfo)
		g.Buddies[chosenBuddyInfo.ID] = append(g.Buddies[chosenBuddyInfo.ID], PlayerInfo{ID: sender.UserID, Username: sender.Username})

		log.Printf("%s chose %s as a buddy. Buddies: %v\n", sender.Username, chosenBuddyInfo.Username, g.Buddies)

		// After choosing a buddy, advance turn
		g.DrawingIndex = (g.DrawingIndex + 1) % len(g.Players)

		log.Printf("Turn advanced to %s (%s) after buddy selection.\n", g.Players[g.DrawingIndex].Username, g.Players[g.DrawingIndex].ID)
		g.broadcastGameState(s)

	case "set_rule":
		if g.CurrentCard == nil || g.CurrentCard.Rank != "K" {
			log.Printf("Cannot set a rule, a King was not just drawn by %s, or no card is drawn.\n", sender.Username)
			return
		}
		if request.NewRule == "" {
			log.Println("No new rule provided.")
			return
		}

		g.CustomRules[sender.UserID] = append(g.CustomRules[sender.UserID], request.NewRule)

		log.Printf("%s set a new rule: \"%s\". Custom Rules: %v\n", sender.Username, request.NewRule, g.CustomRules)

		// After setting a rule, advance turn
		g.DrawingIndex = (g.DrawingIndex + 1) % len(g.Players)
		log.Printf("Turn advanced to %s (%s) after rule setting.\n", g.Players[g.DrawingIndex].Username, g.Players[g.DrawingIndex].ID)
		g.broadcastGameState(s)

	default:
		log.Printf("Unknown game action type: %s from %s\n", request.Type, sender.Username)
	}
}

func (g *KingsCupLogic) ResetState(s *Session) {
	g.mu.Lock()
	g.GameStarted = false
	g.mu.Unlock()

	// Re-initialize the state (reshuffle deck, clear buddies)
	g.InitState(s)
}

type BurnBookLogic struct {
	mu          sync.Mutex
	Timer       *time.Timer
	SkipTimer   chan bool
	Questions   []string
	Phase       string
	Votes       map[int]map[string]int  // [QuestionIndex] -> [CandidateID] -> Count
	WhoVoted    map[int]map[string]bool // [QuestionIndex] -> [VoterID] -> bool
	VotingIndex int
	RevealIndex int
}

type RoundResult struct {
	WinnerID string            `json:"winnerId"` // Helper to identify the top victim easily
	Results  []PlayerRoundInfo `json:"results"`  // List of all players who got votes
}

type PlayerRoundInfo struct {
	UserID string `json:"userId"`
	Votes  int    `json:"votes"`
}

type BurnBookGameState struct {
	Players        []PlayerInfo `json:"players,omitempty"`
	Phase          string       `json:"phase"`
	QuestionText   string       `json:"questionText,omitempty"`
	RoundResults   *RoundResult `json:"roundResults,omitempty"`
	CollectedCount int          `json:"collectedCount,omitempty"`
	TimeRemaining  int          `json:"timeRemaining,omitempty"`
	CurrentNumber  int          `json:"currentNumber,omitempty"`
	TotalQuestions int          `json:"totalQuestions,omitempty"`
	HasVoted       bool         `json:"hasVoted,omitempty"`
}

// func (g *BurnBookLogic) InitState(s *Session) interface{} {
// 	g.mu.Lock()
// 	defer g.mu.Unlock()

// 	g.Phase = "collecting"
// 	g.Questions = make([]string, 0)
// 	g.Votes = make(map[int]map[string]int)
// 	g.WhoVoted = make(map[int]map[string]bool)
// 	g.VotingIndex = 0
// 	g.RevealIndex = -1
// 	g.SkipTimer = make(chan bool)

// 	return BurnBookGameState{
// 		Phase:          "collecting",
// 		CollectedCount: 0,
// 	}
// }

func (g *BurnBookLogic) InitState(s *Session) interface{} {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Phase = "collecting"
	g.Questions = make([]string, 0)
	g.Votes = make(map[int]map[string]int)
	g.WhoVoted = make(map[int]map[string]bool)
	g.VotingIndex = 0
	g.RevealIndex = -1

	// Re-make channel to ensure it's fresh
	g.SkipTimer = make(chan bool)

	return BurnBookGameState{
		Phase:          "collecting",
		CollectedCount: 0,
	}
}

func (g *BurnBookLogic) ResetState(s *Session) {
	g.mu.Lock()
	// 1. Stop any active timer
	if g.Timer != nil {
		g.Timer.Stop()
		g.Timer = nil
	}

	// 2. Non-blocking drain of the channel
	select {
	case <-g.SkipTimer:
	default:
	}
	g.mu.Unlock()

	// 3. Re-initialize
	g.InitState(s)
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

		// Initialize maps if needed
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

		// Record vote
		g.Votes[g.VotingIndex][request.TargetID]++
		g.WhoVoted[g.VotingIndex][sender.UserID] = true

		// --- LOGIC TO CHECK IF ALL PLAYERS VOTED ---
		activePlayers := 0
		for c := range s.Clients {
			if c.Username != "" { // Ensure they are joined
				activePlayers++
			}
		}

		votesCast := len(g.WhoVoted[g.VotingIndex])

		// Broadcast update so clients see "Waiting for others..."
		broadcastVotingState(s, g)

		// If everyone has voted, trigger the skip
		if votesCast >= activePlayers {
			log.Println("All players voted. Skipping timer.")

			// Non-blocking send to avoid deadlocks if timer just fired
			select {
			case g.SkipTimer <- true:
			default:
			}
		}
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

	if g.VotingIndex >= len(g.Questions) {
		g.Phase = "results"
		g.RevealIndex = -1
		broadcast(s, GameStatePayload{
			Action:    "game_update",
			GameState: BurnBookGameState{Phase: "results_wait"},
		})
		g.mu.Unlock()
		return
	}

	broadcastVotingState(s, g)
	currentIndex := g.VotingIndex
	g.mu.Unlock()

	// ASSIGN TO STRUCT FIELD
	g.mu.Lock()
	g.Timer = time.NewTimer(30 * time.Second)
	g.mu.Unlock()

	go func() {
		g.mu.Lock()
		timer := g.Timer
		g.mu.Unlock()

		if timer == nil {
			return
		} // Guard against nil

		select {
		case <-timer.C:
			// Time naturally expired
		case <-g.SkipTimer:
			// Force stop
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}

		g.mu.Lock()
		// Safety check: Did the game reset while we were waiting?
		// If Phase is 'collecting', we were reset.
		if g.Phase != "voting" {
			g.mu.Unlock()
			return
		}

		if g.VotingIndex == currentIndex {
			g.VotingIndex++
			g.mu.Unlock()
			g.startQuestionTimer(s)
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

const (
	ROLE_MAFIA    = "MAFIA"
	ROLE_DOCTOR   = "DOCTOR"
	ROLE_POLICE   = "POLICE"
	ROLE_SPY      = "SPY"
	ROLE_WHORE    = "WHORE"
	ROLE_CIVILIAN = "CIVILIAN"
)

type MafiaGameState struct {
	AlivePlayers []PlayerInfo `json:"alivePlayers"`
	DeadPlayers  []PlayerInfo `json:"deadPlayers"`

	// Detailed breakdown of who voted for whom (visible during DAY)
	Votes map[string]string `json:"votes"`

	Phase         string            `json:"phase"` // "LOBBY", "NIGHT", "DAY", "GAME_OVER"
	Message       string            `json:"message"`
	MyRole        string            `json:"myRole,omitempty"`
	Winner        string            `json:"winner,omitempty"`
	RevealedRoles map[string]string `json:"revealedRoles,omitempty"`
}

type MafiaLogic struct {
	mu sync.Mutex

	Roles   map[string]string // UserID -> Role
	IsAlive map[string]bool   // UserID -> bool

	// NIGHT STATE
	// Maps UserID (Actor) -> TargetID (Victim)
	NightActions map[string]string

	// DAY STATE
	// Maps UserID (Voter) -> TargetID (Accused)
	Votes map[string]string

	Phase string
}

func (g *MafiaLogic) ResetState(s *Session) {
	g.InitState(s)
}

func (g *MafiaLogic) InitState(s *Session) interface{} {
	g.mu.Lock()

	// 1. Check Player Count (Safety check)
	if len(s.Clients) < 3 {
		// Not enough players, remain in lobby state essentially
		g.Phase = "LOBBY"
		Message := "Not enough players to start (Min 3)"
		g.mu.Unlock()
		return MafiaGameState{
			Phase:   "LOBBY",
			Message: Message,
		}
	}

	// 2. Initialize Maps
	g.Phase = "NIGHT"
	g.Roles = make(map[string]string)
	g.IsAlive = make(map[string]bool)
	g.Votes = make(map[string]string)
	g.NightActions = make(map[string]string)

	// 3. Mark everyone alive
	for client := range s.Clients {
		g.IsAlive[client.UserID] = true
	}

	// 4. Assign Roles
	// Note: We hold the lock here because assignRoles writes to g.Roles
	g.assignRoles(s)

	g.mu.Unlock() // Unlock before startNightPhase because it locks internally

	// 5. Start the Night Phase immediately
	// This broadcasts the "NIGHT" state to everyone, effectively skipping the second lobby
	g.startNightPhase(s)

	// Return nil or the state. Since startNightPhase broadcasts,
	// the return value here is less critical, but we return the initial state for consistency.
	return MafiaGameState{
		Phase:   "NIGHT",
		Message: "Night has fallen",
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

	g.mu.Lock()

	// 1. START GAME
	// if payload.Type == "start_game" && sender.IsHost && g.Phase == "NIGHT" {
	// 	if len(s.Clients) < 3 {
	// 		g.mu.Unlock()
	// 		return
	// 	}
	// 	g.assignRoles(s)
	// 	g.IsAlive = make(map[string]bool)
	// 	for client := range s.Clients {
	// 		g.IsAlive[client.UserID] = true
	// 	}
	// 	g.mu.Unlock()
	// 	g.startNightPhase(s)
	// 	return
	// }

	// 2. NIGHT ACTIONS
	if payload.Type == "night_action" && g.Phase == "NIGHT" {
		if !g.IsAlive[sender.UserID] {
			g.mu.Unlock()
			return
		}

		role := g.Roles[sender.UserID]
		// Only special roles act
		if role != ROLE_CIVILIAN {
			g.NightActions[sender.UserID] = payload.TargetID

			// Check if all active roles have submitted their actions
			if g.haveAllNightActionsBeenReceived() {
				g.mu.Unlock()
				g.resolveNight(s) // Transition: Night -> Day
				return
			}
		}
		g.mu.Unlock()
		return
	}

	// 3. DAY VOTING (Resolves Day)
	if payload.Type == "vote" && g.Phase == "DAY" {
		// Validate voter and target are alive
		if !g.IsAlive[sender.UserID] || !g.IsAlive[payload.TargetID] {
			g.mu.Unlock()
			return
		}

		g.Votes[sender.UserID] = payload.TargetID

		// Count alive players to see if everyone has voted
		aliveCount := 0
		for _, alive := range g.IsAlive {
			if alive {
				aliveCount++
			}
		}

		// IF everyone has voted -> Resolve Day immediately
		if len(g.Votes) >= aliveCount {
			g.mu.Unlock()
			g.resolveDay(s) // <--- THIS IS THE CALL
			return
		}

		// If not everyone voted yet, broadcast progress
		g.broadcastState(s, "DAY", fmt.Sprintf("Votes: %d/%d", len(g.Votes), aliveCount))
		g.mu.Unlock()
		return
	}

	g.mu.Unlock()
}

func (g *MafiaLogic) resolveNight(s *Session) {
	g.mu.Lock()

	// 1. Identify who targets whom
	// We need to resolve blocks (Whore) first.
	blockedPlayers := make(map[string]bool)

	// Find the Whore's action
	for actorID, targetID := range g.NightActions {
		if g.Roles[actorID] == ROLE_WHORE {
			blockedPlayers[targetID] = true
			// Whore blocks the target. The target's action will fail.
		}
	}

	// 2. Prepare outcomes
	killedID := ""
	healedID := ""

	// Private info to send back to Police/Spy
	policeResult := ""
	policeRecipient := ""
	// spyResult := ""
	// spyRecipient := ""

	// 3. Process Actions (excluding those who were blocked)
	for actorID, targetID := range g.NightActions {
		if blockedPlayers[actorID] {
			// Inform the blocked player? (Optional, usually they just fail silently or get notified)
			client := g.getClientByID(s, actorID)
			if client != nil {
				g.sendPrivateMessage(client, "system_message", "You were busy fucking by the Whore")
			}
			continue
		}

		role := g.Roles[actorID]
		switch role {
		case ROLE_MAFIA:
			killedID = targetID
		case ROLE_DOCTOR:
			healedID = targetID
		case ROLE_POLICE:
			policeRecipient = actorID
			targetRole := g.Roles[targetID]

			// --- CHANGED POLICE LOGIC ---
			// The Police only detects ROLE_MAFIA.
			// ROLE_SPY appears as "Innocent" (same as Civilians/Doctor/etc).
			isDetected := targetRole == ROLE_MAFIA

			targetUsername := g.getUsername(s, targetID)

			if isDetected {
				policeResult = fmt.Sprintf("%s is MAFIA", targetUsername)
			} else {
				// This covers Civilians, Doctor, Whore, AND SPY
				policeResult = fmt.Sprintf("%s is Innocent", targetUsername)
			}
		}
	}
	// 4. Resolve Life/Death
	finalDeathMsg := "The night was quiet"

	if killedID != "" {
		if killedID == healedID {
			finalDeathMsg = "The Doctor saved the victim."
		} else {
			g.IsAlive[killedID] = false
			finalDeathMsg = fmt.Sprintf("%s was killed in the night", g.getUsername(s, killedID))
		}
	}

	// 5. Send Intel to Police/Spy
	if policeRecipient != "" && policeResult != "" {
		c := g.getClientByID(s, policeRecipient)
		g.sendPrivateMessage(c, "intel", policeResult)
	}


	// if spyRecipient != "" && spyResult != "" {
	// 	c := g.getClientByID(s, spyRecipient)
	// 	g.sendPrivateMessage(c, "intel", spyResult)
	// }

	// 6. Check Win
	if g.checkWinCondition(s) {
		g.mu.Unlock()
		return
	}

	g.mu.Unlock()

	// Start Day
	g.startDayPhase(s, finalDeathMsg)
}

// func (g *MafiaLogic) resolveDay(s *Session) {
// 	g.mu.Lock()

// 	// Tally Votes
// 	voteCounts := make(map[string]int)
// 	for _, target := range g.Votes {
// 		voteCounts[target]++
// 	}

// 	maxVotes := 0
// 	victimID := ""
// 	isTie := false

// 	for target, count := range voteCounts {
// 		if count > maxVotes {
// 			maxVotes = count
// 			victimID = target
// 			isTie = false
// 		} else if count == maxVotes {
// 			isTie = true
// 		}
// 	}

// 	resultMsg := ""
// 	if isTie || maxVotes == 0 {
// 		resultMsg = "Tie vote. No one was executed"
// 	} else {
// 		g.IsAlive[victimID] = false
// 		resultMsg = fmt.Sprintf("The town decided. %s was executed", g.getUsername(s, victimID))
// 	}

// 	if g.checkWinCondition(s) {
// 		g.mu.Unlock()
// 		return
// 	}

// 	g.mu.Unlock()

// 	// Brief pause (logic-wise, we just start night immediately after sending result)
// 	// Or we can wait for Host to click "Next Round" if we wanted,
// 	// but the prompt implied transition on player action (voting done -> Result -> Night).

// 	// We'll send the result, then immediately start night instructions.
// 	// Effectively "Phase Results" is skipped or instantaneous.

// 	// Helper to send message before changing state
// 	g.mu.Lock()
// 	g.broadcastState(s, "RESULTS", resultMsg)
// 	g.mu.Unlock()

//		// Small artificial blocking/delay isn't possible without timers/sleep.
//		// We will just transition to Night immediately.
//		// The client will receive "RESULTS" packet then immediately "NIGHT" packet.
//		g.startNightPhase(s)
//	}

func (g *MafiaLogic) resolveDay(s *Session) {
	g.mu.Lock()

	// 1. Tally Votes & Determine Victim
	voteCounts := make(map[string]int)
	for _, target := range g.Votes {
		voteCounts[target]++
	}

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
		resultMsg = "Tie vote. No one was executed."
	} else {
		g.IsAlive[victimID] = false
		resultMsg = fmt.Sprintf("The town decided. %s was executed.", g.getUsername(s, victimID))
	}

	// 2. BROADCAST RESULT IMMEDIATELY (Do not check win yet)
	g.Phase = "RESULTS"
	g.broadcastState(s, "RESULTS", resultMsg)

	g.mu.Unlock()

	// 3. Wait 5 seconds, THEN check win or start night
	go func() {
		time.Sleep(5 * time.Second)

		g.mu.Lock()
		// Check if the execution ended the game
		if g.checkWinCondition(s) {
			g.mu.Unlock()
			return // Game Over broadcast sent inside checkWinCondition
		}
		g.mu.Unlock()

		// If game not over, proceed to Night
		g.startNightPhase(s)
	}()
}

func (g *MafiaLogic) startDayPhase(s *Session, morningMsg string) {
	g.mu.Lock()
	g.Phase = "DAY"
	g.Votes = make(map[string]string) // Reset votes

	// Broadcast start of day
	g.broadcastState(s, "DAY", morningMsg+" Discuss and Vote")
	g.mu.Unlock()
}
func (g *MafiaLogic) checkWinCondition(s *Session) bool {
	activeMafiaCount := 0 // Only ROLE_MAFIA (The killers)
	mafiaTeamCount := 0   // ROLE_MAFIA + ROLE_SPY (For majority calc)
	civTeamCount := 0

	for id, alive := range g.IsAlive {
		if alive {
			role := g.Roles[id]
			
			if role == ROLE_MAFIA {
				activeMafiaCount++ 
				mafiaTeamCount++
			} else if role == ROLE_SPY {
				// Spy counts for the team size, but is NOT an active killer
				mafiaTeamCount++
			} else {
				civTeamCount++
			}
		}
	}

	winner := ""

	// 1. CIVILIAN WIN CONDITION:
	// If the actual Mafia (the Killer) is dead, the game ends.
	// Even if the Spy is still alive, they cannot kill, so Civilians win.
	if activeMafiaCount == 0 {
		winner = "CIVILIANS"
	} else if mafiaTeamCount >= civTeamCount {
		// 2. MAFIA WIN CONDITION:
		// If Mafia Team (Killer + Spy) outnumbers or equals Civilians
		winner = "MAFIA"
	}

	if winner != "" {
		g.Phase = "GAME_OVER"
		
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
				Phase:         "GAME_OVER",
				Message:       fmt.Sprintf("GAME OVER! %s WIN!", winner),
				AlivePlayers:  alive,
				DeadPlayers:   dead,
				Votes:         g.Votes,
				Winner:        winner,
				RevealedRoles: g.Roles,
			},
		}

		bytes, _ := json.Marshal(payload)
		s.Broadcast <- bytes
		return true
	}
	return false
}

func (g *MafiaLogic) haveAllNightActionsBeenReceived() bool {
	for id, alive := range g.IsAlive {
		if !alive { // If player is dead do nothing
			continue
		}

		role := g.Roles[id]
		
		// Fix: Civilians AND Spies don't act. 
		// Everyone else (Mafia, Doctor, Police, Whore) must submit an action.
		if role != ROLE_CIVILIAN && role != ROLE_SPY {
			if _, ok := g.NightActions[id]; !ok {
				return false // Waiting for this person
			}
		}
	}
	return true
}

func (g *MafiaLogic) startNightPhase(s *Session) {
	g.mu.Lock()
	g.Phase = "NIGHT"
	g.NightActions = make(map[string]string)
	g.Votes = make(map[string]string)

	g.broadcastState(s, "NIGHT", "Night has fallen")

	// 1. Build the list of MAFIA members (Excluding Spy)
	mafiaNames := []string{}
	for id, role := range g.Roles {
        // Only add actual Mafia to this list. DO NOT add the Spy.
		if role == ROLE_MAFIA {
			mafiaNames = append(mafiaNames, g.getUsername(s, id))
		}
	}
	mafiaListStr := strings.Join(mafiaNames, ", ")

	for client := range s.Clients {
		role := g.Roles[client.UserID]
		if !g.IsAlive[client.UserID] {
			continue
		}

		var prompt string
		
		switch role {
		case ROLE_MAFIA:
			prompt = "Choose a player to KILL"
		case ROLE_DOCTOR:
			prompt = "Choose a player to HEAL"
		case ROLE_POLICE:
			prompt = "Choose a player to INVESTIGATE"
		case ROLE_WHORE:
			prompt = "Choose a player to BLOCK"
        // Spy has no active prompt
		default: 
			prompt = "Sleep tight..."
		}

		// 2. Send Action Request
		if prompt != "Sleep tight..." {
			g.sendPrivateMessage(client, "action_request", prompt)
		}

		// 3. Send Intel / Team Knowledge
        
        // CASE A: User is SPY
        // Spy sees the Mafia list.
		if role == ROLE_SPY {
			msg := "The Mafia members are: " + mafiaListStr
			g.sendPrivateMessage(client, "intel", msg)
		}

        // CASE B: User is MAFIA
        // Mafia also needs to know who their teammates are (Standard Mafia rules).
        // Since 'mafiaListStr' only contains ROLE_MAFIA, they will NOT see the Spy.
        if role == ROLE_MAFIA {
            // We use the "intel" channel for this static info too, 
            // so it appears in the blue box on the frontend.
            msg := "Your team is: " + mafiaListStr
            g.sendPrivateMessage(client, "intel", msg)
        }
	}

	g.mu.Unlock()
}

func (g *MafiaLogic) sendPrivateMessage(c *Client, typeStr string, content string) {
	msg := map[string]interface{}{
		"action":  typeStr,
		"content": content,
	}
	data, _ := json.Marshal(msg)
	c.Send <- data
}

func (g *MafiaLogic) broadcastState(s *Session, phase string, msg string) {
	alive := []PlayerInfo{}
	dead := []PlayerInfo{}

	for client := range s.Clients {
		p := PlayerInfo{ID: client.UserID, Username: client.Username}
		if g.IsAlive[client.UserID] { //If the user is alive
			alive = append(alive, p)
		} else { //the user is dead
			dead = append(dead, p)
		}
	}

	payload := GameStatePayload{
		Action: "game_update",
		GameState: MafiaGameState{
			Phase:        phase,
			Message:      msg,
			AlivePlayers: alive,
			DeadPlayers:  dead,
			Votes:        g.Votes,
		},
	}

	bytes, _ := json.Marshal(payload)
	s.Broadcast <- bytes //sending the whole shit to the sessions broadcast channel, which then sends the state to every client
}
func (g *MafiaLogic) assignRoles(s *Session) {
	ids := make([]string, 0, len(s.Clients))
	for client := range s.Clients {
		ids = append(ids, client.UserID)
	}
	count := len(ids)

	rand.NewSource(time.Now().UnixNano())
	rand.Shuffle(count, func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })

	// Default all to CIVILIAN
	for _, id := range ids {
		g.Roles[id] = ROLE_CIVILIAN
	}

	// 1. Always 1 Mafia
	g.Roles[ids[0]] = ROLE_MAFIA

	if count == 4 {
		g.Roles[ids[1]] = ROLE_DOCTOR
	} else if count >= 5 {
		g.Roles[ids[1]] = ROLE_DOCTOR
		g.Roles[ids[2]] = ROLE_POLICE
		g.Roles[ids[3]] = ROLE_SPY
		g.Roles[ids[4]] = ROLE_WHORE
	}

	// --- SEND ROLES TO CLIENTS ---
	for client := range s.Clients {
		role := g.Roles[client.UserID]

		payload := GameStatePayload{
			Action: "game_update",
			GameState: MafiaGameState{
				// CHANGE THIS FROM "LOBBY" TO "NIGHT"
				// This ensures the UI renders the Board immediately with the Role
				Phase:   "NIGHT",
				MyRole:  role,
				Message: "Assigning Roles...",
			},
		}
		bytes, _ := json.Marshal(payload)
		client.Send <- bytes
	}
}

func (g *MafiaLogic) getUsername(s *Session, userID string) string {
	for c := range s.Clients {
		if c.UserID == userID {
			return c.Username
		}
	}
	return "Unknown"
}

func (g *MafiaLogic) getClientByID(s *Session, userID string) *Client {
	for c := range s.Clients {
		if c.UserID == userID {
			return c
		}
	}
	return nil
}
