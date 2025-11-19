package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)


type UserPresence struct {
	UserID      uuid.UUID `json:"user_id"`
	LastSeen    time.Time `json:"last_seen"`
	IsActive    bool      `json:"is_active"`
	DeviceInfo  string    `json:"device_info"`
	AppVersion  string    `json:"app_version"`
	Platform    string    `json:"platform"`
}


type AnalyticsService struct {
	db *pgxpool.Pool
}

func NewAnalyticsService(db *pgxpool.Pool) *AnalyticsService {
	return &AnalyticsService{db: db}
}


func (s *AnalyticsService) UpdatePresence(ctx context.Context, clerkID string, deviceInfo map[string]string) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	deviceInfoJSON, _ := json.Marshal(deviceInfo)

	query := `
		INSERT INTO user_presence (user_id, last_seen, is_active, device_info, app_version, platform)
		VALUES ($1, NOW(), true, $2, $3, $4)
		ON CONFLICT (user_id) 
		DO UPDATE SET 
			last_seen = NOW(), 
			is_active = true,
			device_info = $2,
			app_version = $3,
			platform = $4
	`

	_, err = s.db.Exec(ctx, query, userID, deviceInfoJSON,
		deviceInfo["app_version"], deviceInfo["platform"])
	return err
}

func (s *AnalyticsService) SetUserInactive(ctx context.Context, clerkID string) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	query := `UPDATE user_presence SET is_active = false WHERE user_id = $1`
	_, err = s.db.Exec(ctx, query, userID)
	return err
}

func (s *AnalyticsService) GetActiveUsers(ctx context.Context) (int, error) {
	var count int
	threshold := time.Now().Add(-2 * time.Minute)

	query := `
		SELECT COUNT(*) 
		FROM user_presence 
		WHERE is_active = true AND last_seen > $1
	`

	err := s.db.QueryRow(ctx, query, threshold).Scan(&count)
	return count, err
}

// ============= CRASH & ERROR TRACKING =============

type CrashReport struct {
	ID          uuid.UUID              `json:"id"`
	UserID      uuid.UUID              `json:"user_id"`
	ErrorType   string                 `json:"error_type"`
	ErrorMsg    string                 `json:"error_message"`
	StackTrace  string                 `json:"stack_trace"`
	AppVersion  string                 `json:"app_version"`
	Platform    string                 `json:"platform"`
	OSVersion   string                 `json:"os_version"`
	DeviceModel string                 `json:"device_model"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
}

func (s *AnalyticsService) ReportCrash(ctx context.Context, clerkID string, report *CrashReport) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	metadataJSON, _ := json.Marshal(report.Metadata)

	query := `
		INSERT INTO crash_reports (
			user_id, error_type, error_message, stack_trace,
			app_version, platform, os_version, device_model, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = s.db.Exec(ctx, query, userID, report.ErrorType, report.ErrorMsg,
		report.StackTrace, report.AppVersion, report.Platform,
		report.OSVersion, report.DeviceModel, metadataJSON)
	return err
}

func (s *AnalyticsService) GetCrashRate(ctx context.Context, days int) (float64, error) {
	query := `
		WITH sessions AS (
			SELECT COUNT(DISTINCT session_id) as total_sessions
			FROM analytics_sessions
			WHERE created_at >= NOW() - INTERVAL '1 day' * $1
		),
		crashes AS (
			SELECT COUNT(*) as total_crashes
			FROM crash_reports
			WHERE created_at >= NOW() - INTERVAL '1 day' * $1
		)
		SELECT 
			CASE 
				WHEN s.total_sessions = 0 THEN 0
				ELSE (c.total_crashes::float / s.total_sessions::float) * 100
			END as crash_rate
		FROM sessions s, crashes c
	`

	var crashRate float64
	err := s.db.QueryRow(ctx, query, days).Scan(&crashRate)
	return crashRate, err
}

// ============= PERFORMANCE TRACKING =============

type PerformanceMetric struct {
	UserID       uuid.UUID              `json:"user_id"`
	MetricType   string                 `json:"metric_type"` // "app_start", "screen_load", "api_call"
	MetricName   string                 `json:"metric_name"`
	Duration     int                    `json:"duration_ms"`
	AppVersion   string                 `json:"app_version"`
	Platform     string                 `json:"platform"`
	Metadata     map[string]interface{} `json:"metadata"`
	SuccessCount int                    `json:"success_count"`
	FailureCount int                    `json:"failure_count"`
}

func (s *AnalyticsService) TrackPerformance(ctx context.Context, clerkID string, metric *PerformanceMetric) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	metadataJSON, _ := json.Marshal(metric.Metadata)

	query := `
		INSERT INTO performance_metrics (
			user_id, metric_type, metric_name, duration_ms,
			app_version, platform, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = s.db.Exec(ctx, query, userID, metric.MetricType, metric.MetricName,
		metric.Duration, metric.AppVersion, metric.Platform, metadataJSON)
	return err
}

func (s *AnalyticsService) GetAveragePerformance(ctx context.Context, metricType string, days int) (map[string]float64, error) {
	query := `
		SELECT 
			metric_name,
			AVG(duration_ms) as avg_duration,
			PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY duration_ms) as p50,
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY duration_ms) as p95
		FROM performance_metrics
		WHERE metric_type = $1 
			AND created_at >= NOW() - INTERVAL '1 day' * $2
		GROUP BY metric_name
	`

	rows, err := s.db.Query(ctx, query, metricType, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]float64)
	for rows.Next() {
		var name string
		var avg, p50, p95 float64
		if err := rows.Scan(&name, &avg, &p50, &p95); err != nil {
			return nil, err
		}
		results[name+"_avg"] = avg
		results[name+"_p50"] = p50
		results[name+"_p95"] = p95
	}

	return results, nil
}

// ============= USER ENGAGEMENT & RETENTION =============

type SessionData struct {
	SessionID  uuid.UUID `json:"session_id"`
	UserID     uuid.UUID `json:"user_id"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    *time.Time `json:"ended_at"`
	Duration   int       `json:"duration_seconds"`
	ScreenViews int      `json:"screen_views"`
	AppVersion string    `json:"app_version"`
	Platform   string    `json:"platform"`
}

func (s *AnalyticsService) StartSession(ctx context.Context, clerkID string, sessionID uuid.UUID, deviceInfo map[string]string) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	query := `
		INSERT INTO analytics_sessions (
			session_id, user_id, started_at, app_version, platform
		) VALUES ($1, $2, NOW(), $3, $4)
	`

	_, err = s.db.Exec(ctx, query, sessionID, userID,
		deviceInfo["app_version"], deviceInfo["platform"])
	return err
}

func (s *AnalyticsService) EndSession(ctx context.Context, sessionID uuid.UUID, screenViews int) error {
	query := `
		UPDATE analytics_sessions
		SET ended_at = NOW(),
			duration_seconds = EXTRACT(EPOCH FROM (NOW() - started_at))::int,
			screen_views = $2
		WHERE session_id = $1
	`

	_, err := s.db.Exec(ctx, query, sessionID, screenViews)
	return err
}

func (s *AnalyticsService) GetDAU(ctx context.Context, date time.Time) (int, error) {
	query := `
		SELECT COUNT(DISTINCT user_id)
		FROM analytics_sessions
		WHERE DATE(started_at) = DATE($1)
	`

	var count int
	err := s.db.QueryRow(ctx, query, date).Scan(&count)
	return count, err
}

func (s *AnalyticsService) GetMAU(ctx context.Context, month time.Time) (int, error) {
	query := `
		SELECT COUNT(DISTINCT user_id)
		FROM analytics_sessions
		WHERE started_at >= DATE_TRUNC('month', $1::date)
			AND started_at < DATE_TRUNC('month', $1::date) + INTERVAL '1 month'
	`

	var count int
	err := s.db.QueryRow(ctx, query, month).Scan(&count)
	return count, err
}

func (s *AnalyticsService) GetRetentionRate(ctx context.Context, cohortDate time.Time, dayN int) (float64, error) {
	query := `
		WITH cohort AS (
			SELECT DISTINCT user_id
			FROM analytics_sessions
			WHERE DATE(started_at) = DATE($1)
		),
		returned AS (
			SELECT DISTINCT s.user_id
			FROM analytics_sessions s
			INNER JOIN cohort c ON s.user_id = c.user_id
			WHERE DATE(s.started_at) = DATE($1) + INTERVAL '1 day' * $2
		)
		SELECT 
			CASE 
				WHEN COUNT(DISTINCT c.user_id) = 0 THEN 0
				ELSE (COUNT(DISTINCT r.user_id)::float / COUNT(DISTINCT c.user_id)::float) * 100
			END as retention_rate
		FROM cohort c
		LEFT JOIN returned r ON c.user_id = r.user_id
	`

	var rate float64
	err := s.db.QueryRow(ctx, query, cohortDate, dayN).Scan(&rate)
	return rate, err
}

// ============= SCREEN FLOW TRACKING =============

type ScreenView struct {
	UserID       uuid.UUID              `json:"user_id"`
	SessionID    uuid.UUID              `json:"session_id"`
	ScreenName   string                 `json:"screen_name"`
	PrevScreen   *string                `json:"previous_screen"`
	Duration     int                    `json:"duration_seconds"`
	Metadata     map[string]interface{} `json:"metadata"`
}

func (s *AnalyticsService) TrackScreenView(ctx context.Context, clerkID string, view *ScreenView) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	metadataJSON, _ := json.Marshal(view.Metadata)

	query := `
		INSERT INTO screen_views (
			user_id, session_id, screen_name, previous_screen,
			duration_seconds, metadata
		) VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err = s.db.Exec(ctx, query, userID, view.SessionID, view.ScreenName,
		view.PrevScreen, view.Duration, metadataJSON)
	return err
}

func (s *AnalyticsService) GetTopScreens(ctx context.Context, days int, limit int) (map[string]int, error) {
	query := `
		SELECT screen_name, COUNT(*) as view_count
		FROM screen_views
		WHERE created_at >= NOW() - INTERVAL '1 day' * $1
		GROUP BY screen_name
		ORDER BY view_count DESC
		LIMIT $2
	`

	rows, err := s.db.Query(ctx, query, days, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[string]int)
	for rows.Next() {
		var screenName string
		var count int
		if err := rows.Scan(&screenName, &count); err != nil {
			return nil, err
		}
		results[screenName] = count
	}

	return results, nil
}

func (s *AnalyticsService) GetScreenFlow(ctx context.Context, days int) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			previous_screen,
			screen_name,
			COUNT(*) as transition_count,
			AVG(duration_seconds) as avg_time_spent
		FROM screen_views
		WHERE created_at >= NOW() - INTERVAL '1 day' * $1
			AND previous_screen IS NOT NULL
		GROUP BY previous_screen, screen_name
		ORDER BY transition_count DESC
		LIMIT 50
	`

	rows, err := s.db.Query(ctx, query, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flows []map[string]interface{}
	for rows.Next() {
		var prevScreen, currScreen string
		var count int
		var avgTime float64
		if err := rows.Scan(&prevScreen, &currScreen, &count, &avgTime); err != nil {
			return nil, err
		}
		flows = append(flows, map[string]interface{}{
			"from":  prevScreen,
			"to":    currScreen,
			"count": count,
			"avg_duration": avgTime,
		})
	}

	return flows, nil
}

// ============= EVENT TRACKING =============

type Event struct {
	UserID     uuid.UUID              `json:"user_id"`
	SessionID  uuid.UUID              `json:"session_id"`
	EventName  string                 `json:"event_name"`
	EventType  string                 `json:"event_type"` // "action", "navigation", "conversion"
	Properties map[string]interface{} `json:"properties"`
}

func (s *AnalyticsService) TrackEvent(ctx context.Context, clerkID string, event *Event) error {
	var userID uuid.UUID
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	propertiesJSON, _ := json.Marshal(event.Properties)

	query := `
		INSERT INTO analytics_events (
			user_id, session_id, event_name, event_type, properties
		) VALUES ($1, $2, $3, $4, $5)
	`

	_, err = s.db.Exec(ctx, query, userID, event.SessionID,
		event.EventName, event.EventType, propertiesJSON)
	return err
}

// ============= CLEANUP JOBS =============

func (s *AnalyticsService) CleanupStalePresence(ctx context.Context) error {
	threshold := time.Now().Add(-2 * time.Minute)
	query := `UPDATE user_presence SET is_active = false WHERE last_seen < $1 AND is_active = true`
	result, err := s.db.Exec(ctx, query, threshold)
	if err != nil {
		return err
	}
	
	rows := result.RowsAffected()
	if rows > 0 {
		log.Printf("Marked %d users as inactive", rows)
	}
	return nil
}

func (s *AnalyticsService) StartCleanupJob() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for range ticker.C {
			ctx := context.Background()
			if err := s.CleanupStalePresence(ctx); err != nil {
				log.Printf("Cleanup job error: %v", err)
			}
		}
	}()
}