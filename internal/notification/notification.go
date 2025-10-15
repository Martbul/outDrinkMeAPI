package notification

import (
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	NotificationFriendRequest NotificationType = "friend_request"
	NotificationAchievement   NotificationType = "achievement"
	NotificationChallenge     NotificationType = "challenge"
	NotificationBattle        NotificationType = "battle"
	NotificationOvertake      NotificationType = "overtake"
	NotificationStreakRisk    NotificationType = "streak_risk"
)

type Notification struct {
	ID        uuid.UUID        `json:"id" db:"id"`
	UserID    uuid.UUID        `json:"user_id" db:"user_id"`
	Type      NotificationType `json:"type" db:"type"`
	Title     string           `json:"title" db:"title"`
	Message   string           `json:"message" db:"message"`
	IsRead    bool             `json:"is_read" db:"is_read"`
	Data      map[string]any   `json:"data" db:"data"`
	CreatedAt time.Time        `json:"created_at" db:"created_at"`
}