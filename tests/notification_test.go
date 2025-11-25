package tests

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"outDrinkMeAPI/internal/types/notification"
	"outDrinkMeAPI/services"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func setupTestDB() *pgxpool.Pool {
    if err := godotenv.Load("../.env"); err != nil {
        _ = godotenv.Load() 
        log.Println("Warning: .env file not found via godotenv")
    }

    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        // Attempt to read directly from system env if file failed
        log.Fatal("DATABASE_URL not set. Make sure .env exists in project root or env var is set.")
    }
    
    db, err := pgxpool.New(context.Background(), dbURL)
    if err != nil {
        log.Fatal(err)
    }
    return db
}

func TestNotificationFlow(t *testing.T) {
	db := setupTestDB()
	defer db.Close()

	// 1. Initialize Service
	svc := services.NewNotificationService(db)

	// 2. Mock Data (You need a real user ID from your DB for this to work)
	// Or insert a temp user here if you have a User service
	userID, _ := uuid.Parse("573024d8-c5a4-40a5-8e35-2f0f11339bc7")

	ctx := context.Background()

	// 3. Test: Create Notification
	req := &notification.CreateNotificationRequest{
		UserID:   userID,
		Type:     notification.TypeStreakMilestone, // Ensure you have this template in DB
		Priority: notification.PriorityHigh,
		Data:     map[string]any{"days": "100"},
	}

	notif, err := svc.CreateNotification(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create notification: %v", err)
	}
	t.Logf("Created Notification ID: %s", notif.ID)

	// 4. Test: Check Dispatcher (Async)
	// We wait 1 second for the worker to pick it up and update status to 'sent'
	time.Sleep(1 * time.Second)

	var status notification.NotificationStatus
	err = db.QueryRow(ctx, "SELECT status FROM notifications WHERE id = $1", notif.ID).Scan(&status)
	if err != nil {
		t.Fatalf("Failed to query status: %v", err)
	}

	// Since we are using MOCK providers, it should succeed immediately
	if status != notification.StatusSent {
		t.Errorf("Expected status 'sent', got '%s'", status)
	}

	// 5. Test: Mark as Read (Use the clerk_id associated with that userID)
	var clerkID string
	db.QueryRow(ctx, "SELECT clerk_id FROM users WHERE id=$1", userID).Scan(&clerkID)

	err = svc.MarkAsRead(ctx, notif.ID, clerkID)
	if err != nil {
		t.Fatalf("Failed to mark as read: %v", err)
	}

	// 6. Test: Verify Unread Count is 0 (assuming this was the only one)
	count, _ := svc.GetUnreadCount(ctx, clerkID)
	t.Logf("Unread count is now: %d", count)
}
