package services

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"strings"
// 	"time"

// 	"github.com/google/uuid"
// 	"github.com/jackc/pgx/v5"
// 	"github.com/jackc/pgx/v5/pgxpool"

// 	"outDrinkMeAPI/internal/notification"
// )

// type NotificationService struct {
// 	db         *pgxpool.Pool
// 	dispatcher *NotificationDispatcher
// }

// func NewNotificationService(db *pgxpool.Pool) *NotificationService {
// 	service := &NotificationService{
// 		db: db,
// 	}

// 	// Initialize dispatcher with the service
// 	service.dispatcher = NewNotificationDispatcher(service)

// 	return service
// }

// // Create a new notification
// func (s *NotificationService) CreateNotification(ctx context.Context, req *notification.CreateNotificationRequest) (*notification.Notification, error) {
// 	// Get template for this notification type
// 	template, err := s.getTemplate(ctx, req.Type)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get template: %w", err)
// 	}

// 	// Render title and body from template
// 	title := s.renderTemplate(template.TitleTemplate, req.Data)
// 	body := s.renderTemplate(template.BodyTemplate, req.Data)

// 	// Set priority if not specified
// 	priority := req.Priority
// 	if priority == "" {
// 		priority = template.DefaultPriority
// 	}

// 	// Calculate expiry
// 	expiresAt := time.Now().Add(time.Duration(template.TTLHours) * time.Hour)

// 	// Check rate limits
// 	canSend, err := s.checkRateLimit(ctx, req.UserID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to check rate limit: %w", err)
// 	}
// 	if !canSend {
// 		return nil, fmt.Errorf("rate limit exceeded for user")
// 	}

// 	// Check user preferences
// 	prefs, err := s.GetUserPreferences(ctx, req.UserID)
// 	if err != nil {
// 		// Create default preferences if not found
// 		prefs, err = s.createDefaultPreferences(ctx, req.UserID)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to create preferences: %w", err)
// 		}
// 	}

// 	// Check if user has this notification type enabled
// 	if enabledTypes, ok := prefs.EnabledTypes[string(req.Type)]; ok && !enabledTypes {
// 		log.Printf("Notification type %s disabled for user %s", req.Type, req.UserID)
// 		return nil, nil // Silently skip
// 	}

// 	// Check quiet hours
// 	if s.isInQuietHours(prefs) && priority != notification.PriorityUrgent {
// 		// Schedule for after quiet hours
// 		scheduledFor := s.calculateAfterQuietHours(prefs)
// 		req.ScheduledFor = &scheduledFor
// 	}

// 	// Create notification
// 	dataJSON, _ := json.Marshal(req.Data)

// 	query := `
// 		INSERT INTO notifications (
// 			user_id, type, priority, status, title, body, data, 
// 			actor_id, scheduled_for, action_url, expires_at
// 		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
// 		RETURNING id, user_id, type, priority, status, title, body, data, 
// 				  actor_id, scheduled_for, sent_at, read_at, failed_at, 
// 				  failure_reason, retry_count, action_url, created_at, expires_at
// 	`

// 	notif := &notification.Notification{}
// 	var dataStr string

// 	err = s.db.QueryRow(
// 		ctx, query,
// 		req.UserID, req.Type, priority, notification.StatusPending,
// 		title, body, dataJSON, req.ActorID, req.ScheduledFor,
// 		req.ActionURL, expiresAt,
// 	).Scan(
// 		&notif.ID, &notif.UserID, &notif.Type, &notif.Priority, &notif.Status,
// 		&notif.Title, &notif.Body, &dataStr, &notif.ActorID, &notif.ScheduledFor,
// 		&notif.SentAt, &notif.ReadAt, &notif.FailedAt, &notif.FailureReason,
// 		&notif.RetryCount, &notif.ActionURL, &notif.CreatedAt, &notif.ExpiresAt,
// 	)

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create notification: %w", err)
// 	}

// 	json.Unmarshal([]byte(dataStr), &notif.Data)

// 	// Increment rate limit counter
// 	s.incrementRateLimit(ctx, req.UserID)

// 	// If not scheduled, send immediately
// 	if req.ScheduledFor == nil {
// 		go s.dispatcher.DispatchNotification(context.Background(), notif, prefs)
// 	}

// 	return notif, nil
// }

// // Get user's notifications with pagination
// func (s *NotificationService) GetNotifications(ctx context.Context, clerkID string, page, pageSize int, unreadOnly bool) (*notification.NotificationListResponse, error) {
// 	var userID uuid.UUID
// 	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
// 	if err != nil {
// 		return nil, fmt.Errorf("user not found: %w", err)
// 	}

// 	offset := (page - 1) * pageSize

// 	whereClause := "WHERE user_id = $1"
// 	if unreadOnly {
// 		whereClause += " AND read_at IS NULL"
// 	}

// 	query := fmt.Sprintf(`
// 		SELECT id, user_id, type, priority, status, title, body, data, 
// 			   actor_id, scheduled_for, sent_at, read_at, failed_at, 
// 			   failure_reason, retry_count, action_url, created_at, expires_at
// 		FROM notifications
// 		%s
// 		ORDER BY created_at DESC
// 		LIMIT $2 OFFSET $3
// 	`, whereClause)

// 	rows, err := s.db.Query(ctx, query, userID, pageSize, offset)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to fetch notifications: %w", err)
// 	}
// 	defer rows.Close()

// 	var notifications []*notification.Notification
// 	for rows.Next() {
// 		notif := &notification.Notification{}
// 		var dataStr string

// 		err := rows.Scan(
// 			&notif.ID, &notif.UserID, &notif.Type, &notif.Priority, &notif.Status,
// 			&notif.Title, &notif.Body, &dataStr, &notif.ActorID, &notif.ScheduledFor,
// 			&notif.SentAt, &notif.ReadAt, &notif.FailedAt, &notif.FailureReason,
// 			&notif.RetryCount, &notif.ActionURL, &notif.CreatedAt, &notif.ExpiresAt,
// 		)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to scan notification: %w", err)
// 		}

// 		json.Unmarshal([]byte(dataStr), &notif.Data)
// 		notifications = append(notifications, notif)
// 	}

// 	// Get counts
// 	var unreadCount, totalCount int
// 	s.db.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL", userID).Scan(&unreadCount)
// 	s.db.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE user_id = $1", userID).Scan(&totalCount)

// 	return &notification.NotificationListResponse{
// 		Notifications: notifications,
// 		UnreadCount:   unreadCount,
// 		TotalCount:    totalCount,
// 		Page:          page,
// 		PageSize:      pageSize,
// 	}, nil
// }

// // Mark notification as read
// func (s *NotificationService) MarkAsRead(ctx context.Context, notificationID uuid.UUID, clerkID string) error {
// 	var userID uuid.UUID
// 	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
// 	if err != nil {
// 		return fmt.Errorf("user not found: %w", err)
// 	}

// 	query := `
// 		UPDATE notifications
// 		SET read_at = NOW(), status = $1
// 		WHERE id = $2 AND user_id = $3 AND read_at IS NULL
// 	`

// 	result, err := s.db.Exec(ctx, query, notification.StatusRead, notificationID, userID)
// 	if err != nil {
// 		return fmt.Errorf("failed to mark as read: %w", err)
// 	}

// 	if result.RowsAffected() == 0 {
// 		return fmt.Errorf("notification not found or already read")
// 	}

// 	return nil
// }

// // Mark all notifications as read
// func (s *NotificationService) MarkAllAsRead(ctx context.Context, clerkID string) error {
// 	var userID uuid.UUID
// 	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
// 	if err != nil {
// 		return fmt.Errorf("user not found: %w", err)
// 	}

// 	query := `
// 		UPDATE notifications
// 		SET read_at = NOW(), status = $1
// 		WHERE user_id = $2 AND read_at IS NULL
// 	`

// 	_, err = s.db.Exec(ctx, query, notification.StatusRead, userID)
// 	return err
// }

// // Delete notification
// func (s *NotificationService) DeleteNotification(ctx context.Context, notificationID uuid.UUID, clerkID string) error {
// 	var userID uuid.UUID
// 	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
// 	if err != nil {
// 		return fmt.Errorf("user not found: %w", err)
// 	}
	
// 	query := "DELETE FROM notifications WHERE id = $1 AND user_id = $2"

// 	result, err := s.db.Exec(ctx, query, notificationID, userID)
// 	if err != nil {
// 		return fmt.Errorf("failed to delete notification: %w", err)
// 	}

// 	if result.RowsAffected() == 0 {
// 		return fmt.Errorf("notification not found")
// 	}

// 	return nil
// }

// // Get user preferences
// func (s *NotificationService) GetUserPreferences(ctx context.Context, clerkID string) (*notification.NotificationPreferences, error) {
// 	var userID uuid.UUID
// 	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE clerk_id = $1`, clerkID).Scan(&userID)
// 	if err != nil {
// 		return nil, fmt.Errorf("user not found: %w", err)
// 	}

// 	query := `
// 		SELECT id, user_id, push_enabled, email_enabled, in_app_enabled,
// 			   enabled_types, quiet_hours_enabled, quiet_hours_start, quiet_hours_end,
// 			   quiet_hours_timezone, max_notifications_per_hour, max_notifications_per_day,
// 			   device_tokens, created_at, updated_at
// 		FROM notification_preferences
// 		WHERE user_id = $1
// 	`

// 	prefs := &notification.NotificationPreferences{}
// 	var enabledTypesStr, deviceTokensStr string

// 	err = s.db.QueryRow(ctx, query, userID).Scan(
// 		&prefs.ID, &prefs.UserID, &prefs.PushEnabled, &prefs.EmailEnabled, &prefs.InAppEnabled,
// 		&enabledTypesStr, &prefs.QuietHoursEnabled, &prefs.QuietHoursStart, &prefs.QuietHoursEnd,
// 		&prefs.QuietHoursTimezone, &prefs.MaxNotificationsPerHour, &prefs.MaxNotificationsPerDay,
// 		&deviceTokensStr, &prefs.CreatedAt, &prefs.UpdatedAt,
// 	)

// 	if err != nil {
// 		if err == pgx.ErrNoRows {
// 			return nil, fmt.Errorf("preferences not found")
// 		}
// 		return nil, fmt.Errorf("failed to get preferences: %w", err)
// 	}

// 	json.Unmarshal([]byte(enabledTypesStr), &prefs.EnabledTypes)
// 	json.Unmarshal([]byte(deviceTokensStr), &prefs.DeviceTokens)

// 	return prefs, nil
// }

// // Update user preferences
// func (s *NotificationService) UpdateUserPreferences(ctx context.Context, userID uuid.UUID, req *notification.UpdatePreferencesRequest) (*notification.NotificationPreferences, error) {
// 	// Build dynamic update query
// 	updates := []string{}
// 	args := []interface{}{userID}
// 	argCount := 2

// 	if req.PushEnabled != nil {
// 		updates = append(updates, fmt.Sprintf("push_enabled = $%d", argCount))
// 		args = append(args, *req.PushEnabled)
// 		argCount++
// 	}
// 	if req.EmailEnabled != nil {
// 		updates = append(updates, fmt.Sprintf("email_enabled = $%d", argCount))
// 		args = append(args, *req.EmailEnabled)
// 		argCount++
// 	}
// 	if req.InAppEnabled != nil {
// 		updates = append(updates, fmt.Sprintf("in_app_enabled = $%d", argCount))
// 		args = append(args, *req.InAppEnabled)
// 		argCount++
// 	}
// 	if req.EnabledTypes != nil {
// 		typesJSON, _ := json.Marshal(req.EnabledTypes)
// 		updates = append(updates, fmt.Sprintf("enabled_types = $%d", argCount))
// 		args = append(args, typesJSON)
// 		argCount++
// 	}
// 	if req.QuietHoursEnabled != nil {
// 		updates = append(updates, fmt.Sprintf("quiet_hours_enabled = $%d", argCount))
// 		args = append(args, *req.QuietHoursEnabled)
// 		argCount++
// 	}
// 	// Add more fields as needed...

// 	if len(updates) == 0 {
// 		return s.GetUserPreferences(ctx, userID)
// 	}

// 	query := fmt.Sprintf(`
// 		UPDATE notification_preferences
// 		SET %s, updated_at = NOW()
// 		WHERE user_id = $1
// 		RETURNING id
// 	`, strings.Join(updates, ", "))

// 	var id uuid.UUID
// 	err := s.db.QueryRow(ctx, query, args...).Scan(&id)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to update preferences: %w", err)
// 	}

// 	return s.GetUserPreferences(ctx, userID)
// }

// // Helper methods

// func (s *NotificationService) getTemplate(ctx context.Context, notifType notification.NotificationType) (*notification.NotificationTemplate, error) {
// 	query := `
// 		SELECT id, type, title_template, body_template, default_priority, ttl_hours, created_at, updated_at
// 		FROM notification_templates
// 		WHERE type = $1
// 	`

// 	template := &notification.NotificationTemplate{}
// 	err := s.db.QueryRow(ctx, query, notifType).Scan(
// 		&template.ID, &template.Type, &template.TitleTemplate, &template.BodyTemplate,
// 		&template.DefaultPriority, &template.TTLHours, &template.CreatedAt, &template.UpdatedAt,
// 	)

// 	if err != nil {
// 		return nil, err
// 	}

// 	return template, nil
// }

// func (s *NotificationService) renderTemplate(template string, data map[string]any) string {
// 	result := template
// 	for key, value := range data {
// 		placeholder := fmt.Sprintf("{{%s}}", key)
// 		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
// 	}
// 	return result
// }

// func (s *NotificationService) createDefaultPreferences(ctx context.Context, userID uuid.UUID) (*notification.NotificationPreferences, error) {
// 	query := `
// 		INSERT INTO notification_preferences (user_id)
// 		VALUES ($1)
// 		RETURNING id
// 	`

// 	var id uuid.UUID
// 	err := s.db.QueryRow(ctx, query, userID).Scan(&id)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return s.GetUserPreferences(ctx, userID)
// }

// func (s *NotificationService) checkRateLimit(ctx context.Context, userID uuid.UUID) (bool, error) {
// 	prefs, err := s.GetUserPreferences(ctx, userID)
// 	if err != nil {
// 		return true, nil // Allow if preferences not found
// 	}

// 	// Check hourly limit
// 	hourStart := time.Now().Truncate(time.Hour)
// 	// hourEnd := hourStart.Add(time.Hour)

// 	var hourCount int
// 	query := `
// 		SELECT COALESCE(notification_count, 0)
// 		FROM notification_rate_limits
// 		WHERE user_id = $1 AND window_start = $2
// 	`
// 	s.db.QueryRow(ctx, query, userID, hourStart).Scan(&hourCount)

// 	if hourCount >= prefs.MaxNotificationsPerHour {
// 		return false, nil
// 	}

// 	return true, nil
// }

// func (s *NotificationService) incrementRateLimit(ctx context.Context, userID uuid.UUID) {
// 	hourStart := time.Now().Truncate(time.Hour)
// 	hourEnd := hourStart.Add(time.Hour)

// 	query := `
// 		INSERT INTO notification_rate_limits (user_id, window_start, window_end, notification_count)
// 		VALUES ($1, $2, $3, 1)
// 		ON CONFLICT (user_id, window_start)
// 		DO UPDATE SET notification_count = notification_rate_limits.notification_count + 1
// 	`

// 	s.db.Exec(context.Background(), query, userID, hourStart, hourEnd)
// }

// func (s *NotificationService) isInQuietHours(prefs *notification.NotificationPreferences) bool {
// 	if !prefs.QuietHoursEnabled || prefs.QuietHoursStart == nil || prefs.QuietHoursEnd == nil {
// 		return false
// 	}

// 	loc, _ := time.LoadLocation(prefs.QuietHoursTimezone)
// 	now := time.Now().In(loc)

// 	start := prefs.QuietHoursStart.In(loc)
// 	end := prefs.QuietHoursEnd.In(loc)

// 	currentTime := now.Hour()*60 + now.Minute()
// 	startTime := start.Hour()*60 + start.Minute()
// 	endTime := end.Hour()*60 + end.Minute()

// 	if startTime < endTime {
// 		return currentTime >= startTime && currentTime < endTime
// 	} else {
// 		return currentTime >= startTime || currentTime < endTime
// 	}
// }

// func (s *NotificationService) calculateAfterQuietHours(prefs *notification.NotificationPreferences) time.Time {
// 	if prefs.QuietHoursEnd == nil {
// 		return time.Now()
// 	}

// 	loc, _ := time.LoadLocation(prefs.QuietHoursTimezone)
// 	now := time.Now().In(loc)
// 	end := prefs.QuietHoursEnd.In(loc)

// 	scheduledTime := time.Date(now.Year(), now.Month(), now.Day(), end.Hour(), end.Minute(), 0, 0, loc)
// 	if scheduledTime.Before(now) {
// 		scheduledTime = scheduledTime.Add(24 * time.Hour)
// 	}

// 	return scheduledTime
// }
