package utils

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"outDrinkMeAPI/internal/types/notification"
)

type NotificationCreator interface {
	CreateNotification(ctx context.Context, req *notification.CreateNotificationRequest) (*notification.Notification, error)
}

func FriendPostedStory(db *pgxpool.Pool, notifier NotificationCreator, actorID uuid.UUID, actorName string, storyUrl string, storyId uuid.UUID) {
	log.Printf("DEBUG NOTIF: Starting friend posted story for Actor: %s", actorName)

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
			Type:     notification.TypeFriendPostedStory,
			Priority: notification.PriorityHigh,
			ActorID:  &actorID,
			Data: map[string]any{
				"username":  actorName,
				"story_url": storyUrl,
				"story_id":  storyId,
			},
			ActionURL: nil,
		}

		_, err := notifier.CreateNotification(bgCtx, req)
		if err != nil {
			log.Printf("Failed to create notification for friend %s: %v", friendID, err)
		}
	}
}

func FriendPostedImageToMix(db *pgxpool.Pool, notifier NotificationCreator, actorID uuid.UUID, actorName string, storyUrl string, storyId uuid.UUID) {
	log.Printf("DEBUG: reaction to post: %s", actorName)

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
			Type:     notification.TypeFriendPostedStory,
			Priority: notification.PriorityHigh,
			ActorID:  &actorID,
			Data: map[string]any{
				"username":  actorName,
				"story_url": storyUrl,
				"story_id":  storyId,
			},
			ActionURL: nil,
		}

		_, err := notifier.CreateNotification(bgCtx, req)
		if err != nil {
			log.Printf("Failed to create notification for friend %s: %v", friendID, err)
		}
	}
}


func ReactionToPostMix(db *pgxpool.Pool, notifier NotificationCreator, reactorId uuid.UUID, reactorUsername string, imageURL string, postId uuid.UUID, owerPostId uuid.UUID) {
	log.Printf("DEBUG NOTIF: Starting friend_posted_mix for Actor: %s", reactorUsername)

	bgCtx := context.Background()

	req := &notification.CreateNotificationRequest{
		UserID:   owerPostId,
		Type:     notification.TypeFriendPostedReaction,
		Priority: notification.PriorityHigh,
		ActorID:  &reactorId,
		Data: map[string]any{
			"username":  reactorUsername,
			"image_url": imageURL,
			"post_id":   postId,
		},
		ActionURL: nil,
	}

	_, err := notifier.CreateNotification(bgCtx, req)
	if err != nil {
		log.Printf("Failed to create notification for post owner %s: %v", owerPostId, err)
	}
}
