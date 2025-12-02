package services

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"outDrinkMeAPI/utils"
	"sort"
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

// func (g *KingsCupLogic) InitState(s *Session) interface{} {
// 	g.mu.Lock()
// 	defer g.mu.Unlock()

// 	g.Deck = utils.GenerateNewDeck()
// 	g.CurrentCard = nil
// 	g.DrawingIndex = 0
// 	g.Buddies = make(map[string][]PlayerInfo)
// 	g.CustomRules = make(map[string]string)
// 	g.KingsDrawn = 0
// 	g.LastKingDrinker = ""

// 	g.GameStarted = true

// 	  initialState := KingsCupGameState{
//         CurrentCard:    nil,
//         CardsRemaining: len(g.Deck),
//         GameOver:       false,
//         KingsInCup:     0,
//         GameStarted:    true,
//         Players:        g.Players,
//     }

//     return initialState
// }
// services/kings_cup_logic.go

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

// func (g *KingsCupLogic) broadcastGameState(session *Session, clientCard *ClientCard) {
// 	var currentPlayerTurnID *string
// 	if len(g.Players) > 0 {
// 		currentPlayerTurnID = &g.Players[g.DrawingIndex].ID
// 	}

// 	kingCupDrinkerInfo := g.GetPlayerInfoByID(g.LastKingDrinker)

// 	state := KingsCupGameState{
// 		Players:             g.Players,
// 		CustomRules:         g.CustomRules,
// 		Buddies:             g.Buddies,
// 		CurrentCard:         clientCard,
// 		CardsRemaining:      len(g.Deck),
// 		GameOver:            len(g.Deck) == 0,
// 		CurrentPlayerTurnID: currentPlayerTurnID,
// 		KingsInCup:          g.KingsDrawn,
// 		KingCupDrinker:      kingCupDrinkerInfo,
// 		GameStarted:         g.GameStarted,
// 	}

// 	response := GameStatePayload{
// 		Action:    "game_update",
// 		GameState: state,
// 	}

// 	bytes, err := json.Marshal(response)
// 	if err != nil {
// 		log.Printf("Error marshalling game state: %v", err)
// 		return
// 	}
// 	log.Printf("Broadcasting game state. Current turn: %v, Card: %v, Players: %d",
// 		currentPlayerTurnID, clientCard, len(g.Players))

// 	// Send to the session's broadcast channel
// 	session.Broadcast <- bytes
// }

// Remove the clientCard argument
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

type BurnBookLogic struct {
	mu          sync.Mutex
	Timer       *time.Timer
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

func (g *BurnBookLogic) InitState(s *Session) interface{} {
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

func (g *MafiaLogic) InitState(s *Session) interface{} {
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
