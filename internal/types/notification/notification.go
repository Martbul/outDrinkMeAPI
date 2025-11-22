package notification

import (
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	TypeStreakMilestone      NotificationType = "streak_milestone"
	TypeStreakAtRisk         NotificationType = "streak_at_risk"
	TypeFriendOvertookYou    NotificationType = "friend_overtook_you"
	TypeMentionedInPost      NotificationType = "mentioned_in_post"
	TypeVideoChipsMilestone  NotificationType = "video_chips_milestone"
	TypeChallengeInvite      NotificationType = "challenge_invite"
	TypeChallengeResult      NotificationType = "challenge_result"
	TypeWeeklyRecap          NotificationType = "weekly_recap"
	TypeDrunkThoughtReaction NotificationType = "drunk_thought_reaction"
)

type NotificationPriority string

const (
	PriorityLow    NotificationPriority = "low"
	PriorityMedium NotificationPriority = "medium"
	PriorityHigh   NotificationPriority = "high"
	PriorityUrgent NotificationPriority = "urgent"
)

type NotificationStatus string

const (
	StatusPending NotificationStatus = "pending"
	StatusSent    NotificationStatus = "sent"
	StatusFailed  NotificationStatus = "failed"
	StatusRead    NotificationStatus = "read"
)

type Notification struct {
	ID            uuid.UUID            `json:"id" db:"id"`
	UserID        uuid.UUID            `json:"user_id" db:"user_id"`
	Type          NotificationType     `json:"type" db:"type"`
	Priority      NotificationPriority `json:"priority" db:"priority"`
	Status        NotificationStatus   `json:"status" db:"status"`
	Title         string               `json:"title" db:"title"`
	Body          string               `json:"body" db:"body"`
	Data          map[string]any       `json:"data" db:"data"`
	ActorID       *uuid.UUID           `json:"actor_id,omitempty" db:"actor_id"`
	ScheduledFor  *time.Time           `json:"scheduled_for,omitempty" db:"scheduled_for"`
	SentAt        *time.Time           `json:"sent_at,omitempty" db:"sent_at"`
	ReadAt        *time.Time           `json:"read_at,omitempty" db:"read_at"`
	FailedAt      *time.Time           `json:"failed_at,omitempty" db:"failed_at"`
	FailureReason *string              `json:"failure_reason,omitempty" db:"failure_reason"`
	RetryCount    int                  `json:"retry_count" db:"retry_count"`
	ActionURL     *string              `json:"action_url,omitempty" db:"action_url"`
	CreatedAt     time.Time            `json:"created_at" db:"created_at"`
	ExpiresAt     *time.Time           `json:"expires_at,omitempty" db:"expires_at"`
}

type NotificationPreferences struct {
	ID                      uuid.UUID       `json:"id" db:"id"`
	UserID                  uuid.UUID       `json:"user_id" db:"user_id"`
	PushEnabled             bool            `json:"push_enabled" db:"push_enabled"`
	EmailEnabled            bool            `json:"email_enabled" db:"email_enabled"`
	InAppEnabled            bool            `json:"in_app_enabled" db:"in_app_enabled"`
	EnabledTypes            map[string]bool `json:"enabled_types" db:"enabled_types"`
	QuietHoursEnabled       bool            `json:"quiet_hours_enabled" db:"quiet_hours_enabled"`
	QuietHoursStart         *time.Time      `json:"quiet_hours_start,omitempty" db:"quiet_hours_start"`
	QuietHoursEnd           *time.Time      `json:"quiet_hours_end,omitempty" db:"quiet_hours_end"`
	QuietHoursTimezone      string          `json:"quiet_hours_timezone" db:"quiet_hours_timezone"`
	MaxNotificationsPerHour int             `json:"max_notifications_per_hour" db:"max_notifications_per_hour"`
	MaxNotificationsPerDay  int             `json:"max_notifications_per_day" db:"max_notifications_per_day"`
	DeviceTokens            []DeviceToken   `json:"device_tokens" db:"device_tokens"`
	CreatedAt               time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at" db:"updated_at"`
}

type DeviceToken struct {
	Token    string    `json:"token"`
	Platform string    `json:"platform"` // "ios", "android", "web"
	AddedAt  time.Time `json:"added_at"`
	LastUsed time.Time `json:"last_used"`
}

type NotificationTemplate struct {
	ID              uuid.UUID            `json:"id" db:"id"`
	Type            NotificationType     `json:"type" db:"type"`
	TitleTemplate   string               `json:"title_template" db:"title_template"`
	BodyTemplate    string               `json:"body_template" db:"body_template"`
	DefaultPriority NotificationPriority `json:"default_priority" db:"default_priority"`
	TTLHours        int                  `json:"ttl_hours" db:"ttl_hours"`
	CreatedAt       time.Time            `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at" db:"updated_at"`
}

