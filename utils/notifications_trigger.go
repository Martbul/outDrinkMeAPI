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
func FriendPostedImageToMix(db *pgxpool.Pool, notifier NotificationCreator, actorID uuid.UUID, actorName string, imageURL string) {
	log.Printf("DEBUG NOTIF: Starting friend_posted_mix for Actor: %s", actorName)

	bgCtx := context.Background()

	query := `
		SELECT friend_id FROM friendships WHERE user_id = $1
		UNION
		SELECT user_id FROM friendships WHERE friend_id = $1
	`

	rows, err := db.Query(bgCtx, query, actorID)

	if err != nil {
		log.Printf("Failed to get friends for notification: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var friendID uuid.UUID
		if err := rows.Scan(&friendID); err != nil {
			continue
		}

		req := &notification.CreateNotificationRequest{
			UserID:   friendID,
			Type:     notification.TypeFriendPostedMix,
			Priority: notification.PriorityHigh,
			ActorID:  &actorID,
			Data: map[string]any{
				"username":  actorName,
				"image_url": imageURL,
			},
			ActionURL: nil,
		}

		_, err := notifier.CreateNotification(bgCtx, req)
		if err != nil {
			log.Printf("Failed to create notification for friend %s: %v", friendID, err)
		}
	}
}
