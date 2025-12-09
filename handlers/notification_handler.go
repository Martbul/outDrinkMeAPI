package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"outDrinkMeAPI/internal/types/notification"
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

func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	unreadOnly := r.URL.Query().Get("unread_only") == "true"

	response, err := h.notificationService.GetNotifications(ctx, clerkID, page, pageSize, unreadOnly)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *NotificationHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	count, err := h.notificationService.GetUnreadCount(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]int{"unread_count": count})
}

func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	log.Println("IN  /api/v1/notifications/:id/read")

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	vars := mux.Vars(r)
	notificationID, err := uuid.Parse(vars["id"])
	log.Println(notificationID)
	if err != nil {
		log.Println("Invalid notification ID")
		log.Println(err)
		respondWithError(w, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	err = h.notificationService.MarkAsRead(ctx, notificationID, clerkID)
	if err != nil {
		log.Println("error")
		log.Println(err)
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Notification marked as read"})
}

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

// DELETE /api/v1/notifications/:id
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

// GET /api/v1/notifications/preferences
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

// PUT /api/v1/notifications/preferences
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

// POST /api/v1/notifications/register-device
func (h *NotificationHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req notification.RegisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.notificationService.RegisterDevice(ctx, clerkID, req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Device registered successfully"})
}

func (h *NotificationHandler) SendTestNotification(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	userID, err := h.notificationService.GetUserIDFromClerkID(ctx, clerkID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

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
