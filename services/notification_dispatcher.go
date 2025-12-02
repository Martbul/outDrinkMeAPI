package services

import (
	"context"
	"log"
	"outDrinkMeAPI/internal/types/notification"
	"sync"
	"time"
)
type PushNotificationProvider interface {
	SendPush(ctx context.Context, tokens []notification.DeviceToken, title, body string, data map[string]any) error
}

// NotificationDispatcher handles sending notifications through various channels
type NotificationDispatcher struct {
	service      *NotificationService
	pushProvider PushNotificationProvider 
	workers      int
	jobQueue     chan *DispatchJob
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

type DispatchJob struct {
	Notification *notification.Notification
	Preferences  *notification.NotificationPreferences
}




func NewNotificationDispatcher(service *NotificationService) *NotificationDispatcher {
	dispatcher := &NotificationDispatcher{
		service:  service,
		workers:  5, // 5 workers is plenty for now
		jobQueue: make(chan *DispatchJob, 100),
		stopChan: make(chan struct{}),
	}

	dispatcher.startWorkers()

	// Start scheduled notification processor
	go dispatcher.processScheduledNotifications()

	// Start cleanup job
	go dispatcher.cleanupExpiredNotifications()

	return dispatcher
}

// Allow injecting the real FCM provider from main.go
func (d *NotificationDispatcher) SetPushProvider(provider PushNotificationProvider) {
	d.pushProvider = provider
}

// Start worker pool
func (d *NotificationDispatcher) startWorkers() {
	for i := 0; i < d.workers; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}
}

func (d *NotificationDispatcher) worker(id int) {
	defer d.wg.Done()
	for {
		select {
		case job := <-d.jobQueue:
			d.processJob(job)
		case <-d.stopChan:
			return
		}
	}
}

func (d *NotificationDispatcher) processJob(job *DispatchJob) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	notif := job.Notification
	prefs := job.Preferences

	// 1. Send Push (If enabled, has tokens, and provider exists)
	if prefs.PushEnabled && len(prefs.DeviceTokens) > 0 && d.pushProvider != nil {
		// This calls the code in internal/notification/fcm.go
		err := d.pushProvider.SendPush(ctx, prefs.DeviceTokens, notif.Title, notif.Body, notif.Data)
		
		if err != nil {
			log.Printf("Push failed for user %s: %v", notif.UserID, err)
			d.markAsFailed(ctx, notif.ID.String(), err)
			return
		}
	} else {
		log.Printf("Skipping push: Enabled=%v, Tokens=%d, ProviderSet=%v", 
			prefs.PushEnabled, len(prefs.DeviceTokens), d.pushProvider != nil)
	}

	// 2. Mark as Sent in DB
	d.markAsSent(ctx, notif.ID.String())
}


// Dispatch a notification (add to queue)
func (d *NotificationDispatcher) DispatchNotification(ctx context.Context, notif *notification.Notification, prefs *notification.NotificationPreferences) {
	job := &DispatchJob{
		Notification: notif,
		Preferences:  prefs,
	}

	select {
	case d.jobQueue <- job:
		log.Printf("Notification %s queued for dispatch", notif.ID)
	case <-time.After(5 * time.Second):
		log.Printf("Failed to queue notification %s: queue full", notif.ID)
	}
}

// Process scheduled notifications (runs periodically)
func (d *NotificationDispatcher) processScheduledNotifications() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.processDueNotifications()
		case <-d.stopChan:
			return
		}
	}
}

func (d *NotificationDispatcher) processDueNotifications() {
	ctx := context.Background()

	query := `
		SELECT id, user_id, type, priority, status, title, body, data,
			   actor_id, scheduled_for, action_url, created_at, expires_at
		FROM notifications
		WHERE status = 'pending'
		  AND scheduled_for IS NOT NULL
		  AND scheduled_for <= NOW()
		  AND (expires_at IS NULL OR expires_at > NOW())
		LIMIT 100
	`

	rows, err := d.service.db.Query(ctx, query)
	if err != nil {
		log.Printf("Failed to fetch scheduled notifications: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		notif := &notification.Notification{}
		var dataStr string

		err := rows.Scan(
			&notif.ID, &notif.UserID, &notif.Type, &notif.Priority, &notif.Status,
			&notif.Title, &notif.Body, &dataStr, &notif.ActorID, &notif.ScheduledFor,
			&notif.ActionURL, &notif.CreatedAt, &notif.ExpiresAt,
		)
		if err != nil {
			log.Printf("Failed to scan scheduled notification: %v", err)
			continue
		}

		// Get user preferences
		prefs, err := d.service.GetUserPreferencesByUUID(ctx, notif.UserID)
		if err != nil {
			log.Printf("Failed to get preferences for user %s: %v", notif.UserID, err)
			continue
		}

		// Dispatch
		d.DispatchNotification(ctx, notif, prefs)
		count++
	}

	if count > 0 {
		log.Printf("Processed %d scheduled notifications", count)
	}
}

// Cleanup expired notifications (runs daily)
func (d *NotificationDispatcher) cleanupExpiredNotifications() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.performCleanup()
		case <-d.stopChan:
			return
		}
	}
}

func (d *NotificationDispatcher) performCleanup() {
	ctx := context.Background()

	// Delete expired notifications
	query := `
		DELETE FROM notifications
		WHERE expires_at < NOW()
		  AND status IN ('sent', 'read')
	`

	result, err := d.service.db.Exec(ctx, query)
	if err != nil {
		log.Printf("Failed to cleanup expired notifications: %v", err)
		return
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Cleaned up %d expired notifications", rowsAffected)
	}

	// Delete old read notifications (older than 90 days)
	query = `
		DELETE FROM notifications
		WHERE read_at < NOW() - INTERVAL '90 days'
		  AND status = 'read'
	`

	result, err = d.service.db.Exec(ctx, query)
	if err != nil {
		log.Printf("Failed to cleanup old read notifications: %v", err)
		return
	}

	rowsAffected = result.RowsAffected()
	if rowsAffected > 0 {
		log.Printf("Cleaned up %d old read notifications", rowsAffected)
	}
}

// Mark notification as sent
func (d *NotificationDispatcher) markAsSent(ctx context.Context, notificationID string) {
	query := `
		UPDATE notifications
		SET status = 'sent', sent_at = NOW()
		WHERE id = $1
	`

	_, err := d.service.db.Exec(ctx, query, notificationID)
	if err != nil {
		log.Printf("Failed to mark notification %s as sent: %v", notificationID, err)
	}
}
// Replace the existing markAsFailed function with this updated version
func (d *NotificationDispatcher) markAsFailed(ctx context.Context, notificationID string, err error) {
	// Fix: We now take a single 'error' instead of '[]error'
	failureReason := err.Error()

	query := `
		UPDATE notifications
		SET status = 'failed', failed_at = NOW(), failure_reason = $2, retry_count = retry_count + 1
		WHERE id = $1
	`

	_, dbErr := d.service.db.Exec(ctx, query, notificationID, failureReason)
	if dbErr != nil {
		log.Printf("Failed to mark notification %s as failed: %v", notificationID, dbErr)
	}

	// Schedule retry for high/urgent priority notifications (max 3 retries)
	var retryCount int
	var priority notification.NotificationPriority
	// Note: Ensure you are using the correct package for NotificationPriority
	// It might be 'notification.PriorityHigh' depending on your imports
	d.service.db.QueryRow(ctx, "SELECT retry_count, priority FROM notifications WHERE id = $1", notificationID).Scan(&retryCount, &priority)

	if retryCount < 3 && (priority == notification.PriorityHigh || priority == notification.PriorityUrgent) {
		// Schedule retry in 5 minutes
		retryTime := time.Now().Add(5 * time.Minute)
		d.service.db.Exec(ctx, "UPDATE notifications SET scheduled_for = $2, status = 'pending' WHERE id = $1", notificationID, retryTime)
		log.Printf("Scheduled retry for notification %s at %s", notificationID, retryTime)
	}
}

// Stop the dispatcher gracefully
func (d *NotificationDispatcher) Stop() {
	log.Println("Stopping notification dispatcher...")
	close(d.stopChan)
	d.wg.Wait()
	log.Println("Notification dispatcher stopped")
}

// Mock implementations for testing

type MockPushProvider struct{}

func (m *MockPushProvider) SendPush(ctx context.Context, tokens []notification.DeviceToken, title, body string, data map[string]any) error {
	log.Printf("MOCK PUSH: Sending to %d devices: %s - %s", len(tokens), title, body)
	// In production, integrate with FCM, APNs, etc.
	return nil
}

type MockEmailProvider struct{}

func (m *MockEmailProvider) SendEmail(ctx context.Context, to, subject, body string) error {
	log.Printf("MOCK EMAIL: To %s, Subject: %s", to, subject)
	// In production, integrate with SendGrid, AWS SES, etc.
	return nil
}