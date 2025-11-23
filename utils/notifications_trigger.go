package utils

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	
	"outDrinkMeAPI/internal/types/notification"
)

// 1. Define an interface here. 
// This tells Go: "I don't need the whole Service package, I just need something that has this one method."
type NotificationCreator interface {
	CreateNotification(ctx context.Context, req *notification.CreateNotificationRequest) (*notification.Notification, error)
}

// 2. Accept the DB and the Interface as arguments
func FriendPostedImageToMix(db *pgxpool.Pool, notifier NotificationCreator, actorID uuid.UUID, actorName string) {
	
	// Create a background context so this keeps running even if the HTTP request finishes
	bgCtx := context.Background()

	// A. Find all friends of the user
	rows, err := db.Query(bgCtx, `
            SELECT user_id_2 FROM friends WHERE user_id_1 = $1
            UNION
            SELECT user_id_1 FROM friends WHERE user_id_2 = $1
        `, actorID)

	if err != nil {
		log.Printf("Failed to get friends for notification: %v", err)
		return
	}
	defer rows.Close()

	// B. Loop through friends and send notification
	for rows.Next() {
		var friendID uuid.UUID
		if err := rows.Scan(&friendID); err != nil {
			continue
		}

		// Construct the request
		req := &notification.CreateNotificationRequest{
			UserID:   friendID,
			Type:     notification.TypeFriendPostedMix, // Ensure this constant exists in types
			Priority: notification.PriorityHigh,
			ActorID:  &actorID,
			Data: map[string]any{
				"username":  actorName,
				"image_url": "...", //!TODO: You  want to pass this as an arg too
			},
			ActionURL: nil,
		}

		_, err := notifier.CreateNotification(bgCtx, req)
		if err != nil {
			log.Printf("Failed to create notification for friend %s: %v", friendID, err)
		}
	}
}