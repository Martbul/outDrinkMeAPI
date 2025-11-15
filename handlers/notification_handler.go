package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"outDrinkMeAPI/internal/notification"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type NotificationHandler struct {
	notificationService *services.NotificationService
}

func NewNotificationHandler(notificationService *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
	}
}

// GET /api/v1/notifications - Get user's notifications
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}
	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	unreadOnly := r.URL.Query().Get("unread_only") == "true"

	// Get notifications
	response, err := h.notificationService.GetNotifications(ctx, clerkID, page, pageSize, unreadOnly)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, response)
}

// GET /api/v1/notifications/unread-count - Get unread count
func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var userID uuid.UUID
	err := h.notificationService.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	var unreadCount int
	query := "SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL"
	err = h.notificationService.db.QueryRow(ctx, query, userID).Scan(&unreadCount)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get unread count")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]int{"unread_count": unreadCount})
}

// PUT /api/v1/notifications/:id/read - Mark notification as read
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}


	vars := mux.Vars(r)
	notificationID, err := uuid.Parse(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	err = h.notificationService.MarkAsRead(ctx, notificationID, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Notification marked as read"})
}

// PUT /api/v1/notifications/read-all - Mark all as read
func (h *NotificationHandler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	err := h.notificationService.MarkAllAsRead(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "All notifications marked as read"})
}

// DELETE /api/v1/notifications/:id - Delete notification
func (h *NotificationHandler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}
	
	vars := mux.Vars(r)
	notificationID, err := uuid.Parse(vars["id"])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	err = h.notificationService.DeleteNotification(ctx, notificationID, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Notification deleted"})
}

// GET /api/v1/notifications/preferences - Get notification preferences
func (h *NotificationHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}


	prefs, err := h.notificationService.GetUserPreferences(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, prefs)
}

// PUT /api/v1/notifications/preferences - Update notification preferences
func (h *NotificationHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}


	var req notification.UpdatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	prefs, err := h.notificationService.UpdateUserPreferences(ctx, clerkID, &req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, prefs)
}

// POST /api/v1/notifications/register-device - Register device token for push notifications
func (h *NotificationHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var userID uuid.UUID
	err := h.notificationService.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	var req notification.RegisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get current preferences
	prefs, err := h.notificationService.GetUserPreferences(ctx, userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get preferences")
		return
	}

	// Add new device token (avoid duplicates)
	newToken := notification.DeviceToken{
		Token:    req.Token,
		Platform: req.Platform,
		AddedAt:  time.Now(),
		LastUsed: time.Now(),
	}

	tokenExists := false
	for i, token := range prefs.DeviceTokens {
		if token.Token == req.Token {
			prefs.DeviceTokens[i].LastUsed = time.Now()
			tokenExists = true
			break
		}
	}

	if !tokenExists {
		prefs.DeviceTokens = append(prefs.DeviceTokens, newToken)
	}

	// Update preferences
	tokensJSON, _ := json.Marshal(prefs.DeviceTokens)
	query := `
		UPDATE notification_preferences
		SET device_tokens = $2, updated_at = NOW()
		WHERE user_id = $1
	`

	_, err = h.notificationService.db.Exec(ctx, query, userID, tokensJSON)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to register device")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Device registered successfully"})
}

// POST /api/v1/notifications/test - Test notification (for development)
func (h *NotificationHandler) SendTestNotification(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var userID uuid.UUID
	err := h.notificationService.db.QueryRow(ctx, "SELECT id FROM users WHERE clerk_id = $1", clerkID).Scan(&userID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	// Create test notification
	req := &notification.CreateNotificationRequest{
		UserID:   userID,
		Type:     notification.TypeStreakMilestone,
		Priority: notification.PriorityHigh,
		Data: map[string]any{
			"days": "7",
		},
	}

	notif, err := h.notificationService.CreateNotification(ctx, req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, notif)
}