package utils

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"outDrinkMeAPI/internal/types/notification"
	sidequest "outDrinkMeAPI/internal/types/side_quest"
)

type NotificationCreator interface {
	CreateNotification(ctx context.Context, req *notification.CreateNotificationRequest) (*notification.Notification, error)
}

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

func FriendPostedQuest(db *pgxpool.Pool, notifier NotificationCreator, actorID uuid.UUID, actorName string, newQuest *sidequest.SideQuest) {
	log.Printf("DEBUG NOTIF: Starting friend_posted_quest for Actor: %s", actorName)

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

	//! probably should change
	actionURL := fmt.Sprintf("outdrinkme://quests/%s", newQuest.ID)

	for rows.Next() {
		var friendID uuid.UUID
		if err := rows.Scan(&friendID); err != nil {
			continue
		}

		// 3. Create the Request
		req := &notification.CreateNotificationRequest{
			UserID:   friendID,
			Type:     notification.TypeFriendPostedQuest, // <--- CHANGED FROM MIX TO QUEST
			Priority: notification.PriorityHigh,
			ActorID:  &actorID,
			Data: map[string]any{
				"username":    actorName,
				"quest_title": newQuest.Title,
				"reward":      newQuest.RewardAmount, // Passing reward so template can use it
			},
			ActionURL: &actionURL,
		}

		// 4. Send
		_, err := notifier.CreateNotification(bgCtx, req)
		if err != nil {
			log.Printf("Failed to create notification for friend %s: %v", friendID, err)
		}
	}
}