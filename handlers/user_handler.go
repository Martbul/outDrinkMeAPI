package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"outDrinkMeAPI/internal/types/canvas"
	"outDrinkMeAPI/internal/types/user"
	"outDrinkMeAPI/internal/types/wish"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
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

func (h *UserHandler) FriendDiscoveryDisplayProfile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	friendDiscoveryId := r.URL.Query().Get("friendDiscoveryId")
	if friendDiscoveryId == "" {
		respondWithError(w, http.StatusBadRequest, "Search query parameter 'friendDiscoveryId' is required")
		return
	}

	log.Printf("FriendDiscoveryId Handler: Request from %s to discover profile %s", clerkID, friendDiscoveryId)

	friendDiscoveryDisplayProfile, err := h.userService.FriendDiscoveryDisplayProfile(ctx, clerkID, friendDiscoveryId)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error in getting data for firiend-discovery")
		return
	}

	respondWithJSON(w, http.StatusOK, friendDiscoveryDisplayProfile)
}

func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

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
		respondWithError(w, http.StatusInternalServerError, "Errow while gettin friends")
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

	friendId := r.URL.Query().Get("friendId")
	if friendId == "" {
		respondWithError(w, http.StatusBadRequest, "Search query parameter 'friendId' is required")
		return
	}

	log.Printf("friendId Handler: Request from %s to remove user from friend list %s", clerkID, friendId)

	err := h.userService.RemoveFriend(ctx, clerkID, friendId)
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
		respondWithError(w, http.StatusInternalServerError, "Error while getting discovery users")
		return
	}

	friends, err := h.userService.GetDiscovery(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, friends)
}

func (h *UserHandler) GetLeaderboards(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting friends leaderboard")
		return
	}

	leaderboards, err := h.userService.GetLeaderboards(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, leaderboards)
}

func (h *UserHandler) GetAchievements(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting user's achievements")
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
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	dateStr := r.URL.Query().Get("date")

	var (
		date time.Time
		err  error
	)
	if dateStr != "" {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
			return
		}
	} else {
		date = time.Now().Truncate(24 * time.Hour)
	}

	type Coordinates struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}

	var req struct {
		DrankToday       bool         `json:"drank_today"`
		ImageUrl         *string      `json:"image_url"`
		ImageWidth       *int         `json:"image_width"`
		ImageHeight      *int         `json:"image_height"`
		LocationText     *string      `json:"location_text"`
		LocationCoords   *Coordinates `json:"location_coords"`
		Alcohols         *[]string    `json:"alcohols"`
		MentionedBuddies []struct {
			ClerkID string `json:"clerkId"`
		} `json:"mentioned_buddies"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var clerkIDs []string
	if len(req.MentionedBuddies) > 0 {
		clerkIDs = make([]string, 0, len(req.MentionedBuddies))
		for _, buddy := range req.MentionedBuddies {
			if buddy.ClerkID != "" {
				clerkIDs = append(clerkIDs, buddy.ClerkID)
			}
		}
	}

	var lat, long *float64
	if req.LocationCoords != nil {
		lat = &req.LocationCoords.Latitude
		long = &req.LocationCoords.Longitude
	}

	var alcohols []string
	if req.Alcohols != nil {
		alcohols = *req.Alcohols
	}

	if err := h.userService.AddDrinking(
		ctx,
		clearkID,
		req.DrankToday,
		req.ImageUrl,
		req.ImageWidth,  // New
		req.ImageHeight, // New
		req.LocationText,
		lat,
		long,
		alcohols,
		clerkIDs,
		date,
	); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Drinking activity added successfully"})
}

func (h *UserHandler) GetMemoryWall(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	postID := vars["postId"]

	if postID == "" {
		http.Error(w, "missing postId", http.StatusBadRequest)
		return
	}

	items, err := h.userService.GetMemoryWall(ctx, postID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if items == nil {
		items = []canvas.CanvasItem{}
	}

	respondWithJSON(w, http.StatusOK, items)
}

func (h *UserHandler) AddMemoryToWall(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	var req struct {
		PostId    string               `json:"post_id"`
		WallItems *[]canvas.CanvasItem `json:"wall_items"`
		Reactions *[]canvas.CanvasItem `json:"reactions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var reactions []canvas.CanvasItem
	if req.Reactions != nil {
		reactions = *req.Reactions
	}
	log.Println(reactions)

	if err := h.userService.AddMemoryToWall(ctx, clearkID, req.PostId, req.WallItems, reactions); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Drinking activity added successfully"})
}

func (h *UserHandler) GetMixVideoFeed(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting videos")
		return
	}

	videos, err := h.userService.GetMixVideoFeed(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, videos)
}

func (h *UserHandler) GetAlcoholismChart(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// 1. Get User ID from Context
	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// 2. Get the period filter from URL (e.g. ?period=1M)
	period := r.URL.Query().Get("period")
	// Validate input against allowed values to be safe, or default to 3M
	switch period {
	case "1M", "3M", "6M", "1Y", "ALL":
		// valid
	default:
		period = "3M"
	}

	chartDataBytes, err := h.userService.GetAlcoholismChart(ctx, clerkID, period)
	if err != nil {
		// Log the actual error internally
		// log.Println("Chart error:", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to fetch chart data")
		return
	}

	respondWithJSON(w, http.StatusOK, json.RawMessage(chartDataBytes))
}

func (h *UserHandler) AddMixVideo(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	var req struct {
		VideoUrl string  `json:"video_url"`
		Caption  *string `json:"caption"`
		Duration int     `json:"duration"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.userService.AddMixVideo(ctx, clearkID, req.VideoUrl, req.Caption, req.Duration); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Drinking activity added successfully"})
}

func (h *UserHandler) AddUserFeedback(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	var req struct {
		Category     string `json:"category"`
		FeedbackText string `json:"feedback_text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.userService.AddUserFeedback(ctx, clearkID, req.Category, req.FeedbackText); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Drinking activity added successfully"})
}

func (h *UserHandler) AddChipsToVideo(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting user")
		return
	}

	var req struct {
		VideoID string `json:"video_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.VideoID == "" {
		respondWithError(w, http.StatusBadRequest, "video_id is required")
		return
	}

	if err := h.userService.AddChipsToVideo(ctx, clerkID, req.VideoID); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Chips added successfully"})
}

func (h *UserHandler) RemoveDrinking(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	dateStr := r.URL.Query().Get("date")

	var (
		date time.Time
		err  error
	)

	if dateStr != "" {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
			return
		}
	} else {
		date = time.Now().Truncate(24 * time.Hour)
	}

	if err := h.userService.RemoveDrinking(ctx, clearkID, date); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Drinking activity removed successfully"})
}

func (h *UserHandler) GetWeeklyDaysDrank(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting weekly days drank")
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
		respondWithError(w, http.StatusInternalServerError, "Error while getting monthly days drank")
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
		respondWithError(w, http.StatusInternalServerError, "Error while getting yearly days drank")
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
		respondWithError(w, http.StatusInternalServerError, "Error while getting all time days drank")
		return
	}

	allTimeDaysDrank, err := h.userService.GetAllTimeDaysDrank(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, allTimeDaysDrank)
}

func (h *UserHandler) GetDrunkFriendThoughts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting drunk friends thoughts")
		return
	}

	drunkFriendThoughts, err := h.userService.GetDrunkFriendThoughts(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, drunkFriendThoughts)
}

func (h *UserHandler) GetUserInventory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting user inventory")
		return
	}

	user_inventory, err := h.userService.GetUserInventory(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, user_inventory)
}

func (h *UserHandler) GetUserAlcoholCollection(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting drunk friends thoughts")
		return
	}

	alcoholCollection, err := h.userService.GetUserAlcoholCollection(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, alcoholCollection)
}

func (h *UserHandler) RemoveAlcoholCollectionItem(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting drunk friends thoughts")
		return
	}

	itemIdForRemoval := r.URL.Query().Get("itemId")
	if itemIdForRemoval == "" {
		respondWithError(w, http.StatusBadRequest, "Search query parameter 'itemIdForRemoval' is required")
		return
	}

	success, err := h.userService.RemoveAlcoholCollectionItem(ctx, clearkID, itemIdForRemoval)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, success)
}

func (h *UserHandler) SearchDbAlcohol(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while getting drunk friends thoughts")
		return
	}

	query := r.URL.Query().Get("alcohol_name")
	if query == "" {
		respondWithError(w, http.StatusBadRequest, "Search query parameter 'alcoholName' is required")
		return
	}

	alcoholItem, err := h.userService.SearchDbAlcohol(ctx, clearkID, query)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, alcoholItem)
}

func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while searching users")
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
		respondWithError(w, http.StatusInternalServerError, "Error while getting calendar")
		return
	}

	year := r.URL.Query().Get("year")
	month := r.URL.Query().Get("month")
	displyUserId := r.URL.Query().Get("displyUserId")

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

	calendar, err := h.userService.GetCalendar(ctx, clearkID, yearInt, monthInt, &displyUserId)
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
func (h *UserHandler) GetYourMix(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	page, limit := getPaginationParams(r)

	yourMixData, err := h.userService.GetYourMix(ctx, clerkID, page, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, yourMixData)
}

// 2. GET GLOBAL MIX (Strangers Only)
func (h *UserHandler) GetGlobalMix(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	page, limit := getPaginationParams(r)

	globalMixData, err := h.userService.GetGlobalMix(ctx, clerkID, page, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, globalMixData)
}

func getPaginationParams(r *http.Request) (int, int) {
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20 // Default limit
	} else if limit > 50 {
		limit = 50 // Max limit cap
	}

	return page, limit
}

func (h *UserHandler) GetUserFriendsPosts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userFriendsPosts, err := h.userService.GetUserFriendsPosts(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, userFriendsPosts)
}

func (h *UserHandler) GetMixTimeline(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clearkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	mixTimelineData, err := h.userService.GetMixTimeline(ctx, clearkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, mixTimelineData)
}

func (h *UserHandler) GetDrunkThought(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Get optional date parameter from query string
	dateStr := r.URL.Query().Get("date")

	var (
		date time.Time
		err  error
	)

	if dateStr != "" {
		// Parse user-specified date
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
			return
		}
	} else {
		// Default to today's date
		date = time.Now().Truncate(24 * time.Hour)
	}

	// Get drunk thought for given date
	drunkThought, err := h.userService.GetDrunkThought(ctx, clerkID, date)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Wrap in an object before responding
	response := map[string]interface{}{
		"drunk_thought": drunkThought,
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *UserHandler) AddDrunkThought(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	var req struct {
		DrunkThought string `json:"drunk_thought"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	drunkThought, err := h.userService.AddDrunkThought(ctx, clerkID, req.DrunkThought)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":       "Drinking thought added successfully",
		"drunk_thought": drunkThought,
	})
}

func (h *UserHandler) GetStories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	stories, err := h.userService.GetStories(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, stories)
}

func (h *UserHandler) AddStory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		VideoUrl      string   `json:"video_url"`
		VideoWidth    float64  `json:"width"`
		VideoHeight   float64  `json:"height"`
		VideoDuration float64  `json:"duration"`
		TaggedBuddies []string `json:"tagged_buddies"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	success, err := h.userService.AddStory(
		ctx,
		clerkID,
		req.VideoUrl,
		req.VideoWidth,
		req.VideoHeight,
		req.VideoDuration,
		req.TaggedBuddies,
	)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, map[string]bool{"success": success})
}

func (h *UserHandler) DeleteStory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving user")
		return
	}

	vars := mux.Vars(r)
	storyID := vars["story_id"]

	if storyID == "" {
		respondWithError(w, http.StatusBadRequest, "Missing story ID")
		return
	}

	success, err := h.userService.DeleteStory(ctx, clerkID, storyID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, success)
}

func (h *UserHandler) RelateStory(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	var req struct {
		StoryId string `json:"story_id"`
		Action  string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	success, err := h.userService.RelateStory(ctx, clerkID, req.StoryId, req.Action)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, success)
}

func (h *UserHandler) MarkStoryAsSeen(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	var req struct {
		StoryId string `json:"story_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	success, err := h.userService.MarkStoryAsSeen(ctx, clerkID, req.StoryId)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, success)
}

func (h *UserHandler) GetAllUserStories(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "Error while adding drinking")
		return
	}

	allUserStories, err := h.userService.GetAllUserStories(ctx, clerkID)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, allUserStories)
}

func (h *UserHandler) GetPremiumDetails(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	premiumDetails, err := h.userService.GetPremiumDetails(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not fetch subscription")
		return
	}

	if premiumDetails == nil {
		respondWithJSON(w, http.StatusOK, nil)
		return
	}

	respondWithJSON(w, http.StatusOK, premiumDetails)
}

type QRTokenPayload struct {
	UserID    string `json:"uid"`
	ExpiresAt int64  `json:"exp"`
}

func (h *UserHandler) GenerateDynamicQR(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// 1. Check if user is actually Premium and Active
	premiumStatus, err := h.userService.GetPremiumDetails(ctx, clerkID)
	if err != nil {
		http.Error(w, "Error checking status", http.StatusInternalServerError)
		return
	}
	if premiumStatus == nil || !premiumStatus.IsActive {
		http.Error(w, "User is not premium", http.StatusForbidden)
		return
	}

	expiration := time.Now().Add(2 * time.Minute).Unix()
	payload := QRTokenPayload{
		UserID:    clerkID,
		ExpiresAt: expiration,
	}

	secretKey := os.Getenv("QR_SIGNING_SECRET")
	if secretKey == "" {
		secretKey = "fallback-secret-change-me"
	}

	payloadBytes, _ := json.Marshal(payload)
	payloadStr := base64.RawURLEncoding.EncodeToString(payloadBytes)

	hMac := hmac.New(sha256.New, []byte(secretKey))
	hMac.Write([]byte(payloadStr))
	signature := base64.RawURLEncoding.EncodeToString(hMac.Sum(nil))

	token := fmt.Sprintf("%s.%s", payloadStr, signature)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":     token,
		"expiresAt": fmt.Sprintf("%d", expiration),
	})
}

func (h *UserHandler) GetWishList(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	wishList, err := h.userService.GetWishList(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not fetch wish list")
		return
	}

	respondWithJSON(w, http.StatusOK, wishList)
}

func (h *UserHandler) AddWishItem(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

    var req wish.CreateWishRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondWithError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

	newItem, err := h.userService.AddWishItem(ctx, clerkID, req.Text)
	if err != nil {
        // Check if error is "limit reached"
        if err.Error() == "limit reached" {
            respondWithError(w, http.StatusForbidden, "Monthly limit reached")
            return
        }
		respondWithError(w, http.StatusInternalServerError, "Could not add wish")
		return
	}

	respondWithJSON(w, http.StatusCreated, newItem)
}

func (h *UserHandler) ToggleWishItem(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

    vars := mux.Vars(r)
    itemID := vars["id"]

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

    success, err := h.userService.ToggleWishItem(ctx, clerkID, itemID)
    if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not toggle item")
		return
	}

    respondWithJSON(w, http.StatusOK, map[string]bool{"success": success})
}
func (h *UserHandler) DeleteAccountPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
       <!DOCTYPE html>
<html lang="bg">
<head>
    <meta charset="UTF-8">
    <title>Изтриване на профил</title>
</head>
<body>
    <h1>Изтриване на профил</h1>
    <p>За да изтриете профила си:</p>
    <ol>
        <li>Отворете приложението</li>
        <li>Отидете в Профил</li> 
		         <li>Натиснете "Edit profile"</li>
        <li>Натиснете "Изтрий профил"</li>
    </ol>
    <p>Или изпратете имейл на: martbul01@gmail.com</p>
</body>
</html>
    `)
}

func (h *UserHandler) UpdateAccountPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
       <!DOCTYPE html>
<html lang="bg">
<head>
    <meta charset="UTF-8">
    <title>Изтриване на данни от профил</title>
</head>
<body>
    <h1>Изтриване на данни от профил</h1>
    <p>За да изтриете данни от профила си:</p>
    <ol>
        <li>Отворете приложението</li>
        <li>Отидете в Профил</li>
        <li>Натиснете "Edit profile"</li>
    </ol>
    <p>Или изпратете имейл на: martbul01@gmail.com</p>
</body>
</html>
    `)
}

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
