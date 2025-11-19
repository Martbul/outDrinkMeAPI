package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"outDrinkMeAPI/middleware"
	"outDrinkMeAPI/services"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type AnalyticsHandler struct {
	analyticsService *services.AnalyticsService
}

func NewAnalyticsHandler(analyticsService *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
	}
}

// ============= PRESENCE TRACKING =============

// POST /analytics/presence/heartbeat
func (h *AnalyticsHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		AppVersion  string `json:"app_version"`
		Platform    string `json:"platform"`
		OSVersion   string `json:"os_version"`
		DeviceModel string `json:"device_model"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	deviceInfo := map[string]string{
		"app_version":  req.AppVersion,
		"platform":     req.Platform,
		"os_version":   req.OSVersion,
		"device_model": req.DeviceModel,
	}

	if err := h.analyticsService.UpdatePresence(ctx, clerkID, deviceInfo); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update presence")
		return
	}

	activeUsers, err := h.analyticsService.GetActiveUsers(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get active users")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"active_users": activeUsers,
		"timestamp":    time.Now(),
	})
}

// POST /analytics/presence/disconnect
func (h *AnalyticsHandler) Disconnect(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	if err := h.analyticsService.SetUserInactive(ctx, clerkID); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to disconnect")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Disconnected successfully",
	})
}

// GET /analytics/presence/active
func (h *AnalyticsHandler) GetActiveUsers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	activeUsers, err := h.analyticsService.GetActiveUsers(ctx)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get active users")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"active_users": activeUsers,
		"timestamp":    time.Now(),
	})
}

// ============= SESSION TRACKING =============

// POST /analytics/session/start
func (h *AnalyticsHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		SessionID   string `json:"session_id"`
		AppVersion  string `json:"app_version"`
		Platform    string `json:"platform"`
		OSVersion   string `json:"os_version"`
		DeviceModel string `json:"device_model"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	deviceInfo := map[string]string{
		"app_version":  req.AppVersion,
		"platform":     req.Platform,
		"os_version":   req.OSVersion,
		"device_model": req.DeviceModel,
	}

	if err := h.analyticsService.StartSession(ctx, clerkID, sessionID, deviceInfo); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to start session")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": sessionID,
		"message":    "Session started successfully",
	})
}

// POST /analytics/session/end
func (h *AnalyticsHandler) EndSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// clerkID, ok := middleware.GetClerkID(ctx)
	// if !ok {
	// 	respondWithError(w, http.StatusUnauthorized, "User not authenticated")
	// 	return
	// }

	var req struct {
		SessionID   string `json:"session_id"`
		ScreenViews int    `json:"screen_views"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	if err := h.analyticsService.EndSession(ctx, sessionID, req.ScreenViews); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to end session")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Session ended successfully",
	})
}

// ============= SCREEN TRACKING =============

// POST /analytics/screen
func (h *AnalyticsHandler) TrackScreen(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		SessionID      string                 `json:"session_id"`
		ScreenName     string                 `json:"screen_name"`
		PreviousScreen *string                `json:"previous_screen"`
		DurationSec    int                    `json:"duration_seconds"`
		Metadata       map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	screenView := &services.ScreenView{
		SessionID:  sessionID,
		ScreenName: req.ScreenName,
		PrevScreen: req.PreviousScreen,
		Duration:   req.DurationSec,
		Metadata:   req.Metadata,
	}

	if err := h.analyticsService.TrackScreenView(ctx, clerkID, screenView); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to track screen view")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Screen view tracked successfully",
	})
}

// GET /analytics/screens/top
func (h *AnalyticsHandler) GetTopScreens(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	daysStr := r.URL.Query().Get("days")
	limitStr := r.URL.Query().Get("limit")

	days := 7
	if daysStr != "" {
		if parsed, err := strconv.Atoi(daysStr); err == nil {
			days = parsed
		}
	}

	limit := 20
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	screens, err := h.analyticsService.GetTopScreens(ctx, days, limit)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get top screens")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"screens": screens,
		"period":  days,
	})
}

// GET /analytics/screens/flow
func (h *AnalyticsHandler) GetScreenFlow(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	daysStr := r.URL.Query().Get("days")
	days := 7
	if daysStr != "" {
		if parsed, err := strconv.Atoi(daysStr); err == nil {
			days = parsed
		}
	}

	flows, err := h.analyticsService.GetScreenFlow(ctx, days)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get screen flow")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"flows":  flows,
		"period": days,
	})
}

// ============= EVENT TRACKING =============

// POST /analytics/event
func (h *AnalyticsHandler) TrackEvent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		SessionID  string                 `json:"session_id"`
		EventName  string                 `json:"event_name"`
		EventType  string                 `json:"event_type"`
		Properties map[string]interface{} `json:"properties"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid session ID")
		return
	}

	event := &services.Event{
		SessionID:  sessionID,
		EventName:  req.EventName,
		EventType:  req.EventType,
		Properties: req.Properties,
	}

	if err := h.analyticsService.TrackEvent(ctx, clerkID, event); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to track event")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Event tracked successfully",
	})
}

// ============= PERFORMANCE TRACKING =============

// POST /analytics/performance
func (h *AnalyticsHandler) TrackPerformance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		MetricType  string                 `json:"metric_type"`
		MetricName  string                 `json:"metric_name"`
		DurationMs  int                    `json:"duration_ms"`
		AppVersion  string                 `json:"app_version"`
		Platform    string                 `json:"platform"`
		OSVersion   string                 `json:"os_version"`
		DeviceModel string                 `json:"device_model"`
		Metadata    map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	metric := &services.PerformanceMetric{
		MetricType: req.MetricType,
		MetricName: req.MetricName,
		Duration:   req.DurationMs,
		AppVersion: req.AppVersion,
		Platform:   req.Platform,
		Metadata:   req.Metadata,
	}

	if err := h.analyticsService.TrackPerformance(ctx, clerkID, metric); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to track performance")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Performance metric tracked successfully",
	})
}

// ============= CRASH REPORTING =============

// POST /analytics/crash
func (h *AnalyticsHandler) ReportCrash(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	clerkID, ok := middleware.GetClerkID(ctx)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	var req struct {
		ErrorType    string                 `json:"error_type"`
		ErrorMessage string                 `json:"error_message"`
		StackTrace   string                 `json:"stack_trace"`
		AppVersion   string                 `json:"app_version"`
		Platform     string                 `json:"platform"`
		OSVersion    string                 `json:"os_version"`
		DeviceModel  string                 `json:"device_model"`
		Metadata     map[string]interface{} `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	report := &services.CrashReport{
		ErrorType:   req.ErrorType,
		ErrorMsg:    req.ErrorMessage,
		StackTrace:  req.StackTrace,
		AppVersion:  req.AppVersion,
		Platform:    req.Platform,
		OSVersion:   req.OSVersion,
		DeviceModel: req.DeviceModel,
		Metadata:    req.Metadata,
	}

	if err := h.analyticsService.ReportCrash(ctx, clerkID, report); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to report crash")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{
		"message": "Crash reported successfully",
	})
}

// GET /analytics/crash-rate
func (h *AnalyticsHandler) GetCrashRate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	daysStr := r.URL.Query().Get("days")
	days := 7
	if daysStr != "" {
		if parsed, err := strconv.Atoi(daysStr); err == nil {
			days = parsed
		}
	}

	crashRate, err := h.analyticsService.GetCrashRate(ctx, days)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get crash rate")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"crash_rate": crashRate,
		"period":     days,
	})
}

// ============= METRICS =============

// GET /analytics/metrics/dau
func (h *AnalyticsHandler) GetDAU(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse optional date parameter
	dateStr := r.URL.Query().Get("date")
	var date time.Time
	if dateStr != "" {
		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid date format. Use YYYY-MM-DD")
			return
		}
		date = parsed
	} else {
		date = time.Now()
	}

	dau, err := h.analyticsService.GetDAU(ctx, date)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get DAU")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"dau":  dau,
		"date": date.Format("2006-01-02"),
	})
}

// GET /analytics/metrics/retention
func (h *AnalyticsHandler) GetRetention(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Parse query parameters
	cohortDateStr := r.URL.Query().Get("cohort_date")
	dayNStr := r.URL.Query().Get("day_n")

	// Default values
	cohortDate := time.Now().AddDate(0, 0, -7) // 7 days ago
	dayN := 7

	if cohortDateStr != "" {
		parsed, err := time.Parse("2006-01-02", cohortDateStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid cohort_date format. Use YYYY-MM-DD")
			return
		}
		cohortDate = parsed
	}

	if dayNStr != "" {
		if parsed, err := strconv.Atoi(dayNStr); err == nil {
			dayN = parsed
		}
	}

	rate, err := h.analyticsService.GetRetentionRate(ctx, cohortDate, dayN)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get retention rate")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"retention_rate": rate,
		"cohort_date":    cohortDate.Format("2006-01-02"),
		"day_n":          dayN,
	})
}