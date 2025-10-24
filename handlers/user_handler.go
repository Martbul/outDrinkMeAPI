package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"outDrinkMeAPI/internal/user"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"strings"
	"time"
)

type UserHandler struct {
	userService *services.UserService
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get authenticated Clerk user ID from context
	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	user, err := h.userService.GetUserByClerkID(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get authenticated Clerk user ID from context
	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req user.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.userService.UpdateProfileByClerkID(ctx, clerkID, &req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

func (h *UserHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Get authenticated Clerk user ID
	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	if err := h.userService.DeleteUserByClerkID(ctx, clerkID); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Account deleted successfully"})
}

func (h *UserHandler) GetFriends(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	friends, err := h.userService.GetFriends(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, friends)
}

func (h *UserHandler) AddFriend(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req user.AddFriend
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("AddFriend Handler: Failed to decode request body: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Printf("AddFriend Handler: Request from %s to add friend %s", clerkID, req.FriendId)

	if req.FriendId == "" {
		respondWithError(w, http.StatusBadRequest, "friendId is required")
		return
	}

	err := h.userService.AddFriend(ctx, clerkID, req.FriendId)
	if err != nil {
		log.Printf("AddFriend Handler: Service error: %v", err)
		// Handle specific error cases
		errMsg := err.Error()
		switch {
		case errMsg == "cannot add yourself as a friend" || errMsg == "friendship already exists":
			respondWithError(w, http.StatusBadRequest, errMsg)
		case errMsg == "friend user not found" || strings.Contains(errMsg, "user not found"):
			respondWithError(w, http.StatusNotFound, errMsg)
		default:
			respondWithError(w, http.StatusInternalServerError, "Failed to add friend")
		}
		return
	}

	log.Printf("AddFriend Handler: Successfully added friend")
	respondWithJSON(w, http.StatusCreated, map[string]string{
		"message": "Friend added successfully",
	})
}
func (h *UserHandler) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req user.RemoveFriend
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("RemoveFriend Handler: Failed to decode request body: %v", err)
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Printf("RemoveFriend Handler: Request from %s to remove friend %s", clerkID, req.FriendId)

	if req.FriendId == "" {
		respondWithError(w, http.StatusBadRequest, "friendId is required")
		return
	}

	err := h.userService.RemoveFriend(ctx, clerkID, req.FriendId)
	if err != nil {
		log.Printf("RemoveFriend Handler: Service error: %v", err)
		errMsg := err.Error()
		switch {
		case errMsg == "friendship not found":
			respondWithError(w, http.StatusNotFound, errMsg)
		case errMsg == "friend user not found" || strings.Contains(errMsg, "user not found"):
			respondWithError(w, http.StatusNotFound, errMsg)
		default:
			respondWithError(w, http.StatusInternalServerError, "Failed to remove friend")
		}
		return
	}

	log.Printf("RemoveFriend Handler: Successfully removed friend")
	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Friend removed successfully",
	})
}

func (h *UserHandler) GetDiscovery(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	friends, err := h.userService.GetDiscovery(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, friends)
}

func (h *UserHandler) GetFriendsLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	friendsLeaderboard, err := h.userService.GetFriendsLeaderboard(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, friendsLeaderboard)
}

func (h *UserHandler) GetGlobalLeaderboard(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	globalLeaderboard, err := h.userService.GetGlobalLeaderboard(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, globalLeaderboard)
}

func (h *UserHandler) GetAchievements(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	achievements, err := h.userService.GetAchievements(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, achievements)
}

func (h *UserHandler) AddDrinking(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		DrankToday bool `json:"drank_today"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.userService.AddDrinking(ctx, clearkID, req.DrankToday); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Drinking activity added successfully"})
}

func (h *UserHandler) GetWeeklyDaysDrank(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	weeklyDaysDrank, err := h.userService.GetWeeklyDaysDrank(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, weeklyDaysDrank)
}

func (h *UserHandler) GetMonthlyDaysDrank(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	monthlyDaysDrank, err := h.userService.GetMonthlyDaysDrank(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, monthlyDaysDrank)
}

func (h *UserHandler) GetYearlyDaysDrank(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	yearlyDaysDrank, err := h.userService.GetYearlyDaysDrank(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, yearlyDaysDrank)
}

func (h *UserHandler) GetAllTimeDaysDrank(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	allTimeDaysDrank, err := h.userService.GetAllTimeDaysDrank(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, allTimeDaysDrank)
}


func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		respondWithError(w, http.StatusBadRequest, "Search query parameter 'q' is required")
		return
	}

	users, err := h.userService.SearchUsers(ctx, clearkID, query)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, users)

}

func (h *UserHandler) GetCalendar(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	year := r.URL.Query().Get("year")
	month := r.URL.Query().Get("month")

	if year == "" || month == "" {
		http.Error(w, "year and month are required", http.StatusBadRequest)
		return
	}

	var yearInt, monthInt int
	if _, err := fmt.Sscanf(year, "%d", &yearInt); err != nil {
		http.Error(w, "invalid year format", http.StatusBadRequest)
		return
	}
	if _, err := fmt.Sscanf(month, "%d", &monthInt); err != nil {
		http.Error(w, "invalid month format", http.StatusBadRequest)
		return
	}

	calendar, err := h.userService.GetCalendar(ctx, clearkID, yearInt, monthInt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, calendar)
}

func (h *UserHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	stats, err := h.userService.GetUserStats(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, stats)
}

// Helper functions
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}
