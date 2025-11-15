package notification

import (
	"time"

	"github.com/google/uuid"
)

type CreateNotificationRequest struct {
	UserID       uuid.UUID            `json:"user_id" validate:"required"`
	Type         NotificationType     `json:"type" validate:"required"`
	Priority     NotificationPriority `json:"priority"`
	Data         map[string]any       `json:"data"`
	ActorID      *uuid.UUID           `json:"actor_id,omitempty"`
	ScheduledFor *time.Time           `json:"scheduled_for,omitempty"`
	ActionURL    *string              `json:"action_url,omitempty"`
}

type UpdatePreferencesRequest struct {
	PushEnabled             *bool           `json:"push_enabled,omitempty"`
	EmailEnabled            *bool           `json:"email_enabled,omitempty"`
	InAppEnabled            *bool           `json:"in_app_enabled,omitempty"`
	EnabledTypes            map[string]bool `json:"enabled_types,omitempty"`
	QuietHoursEnabled       *bool           `json:"quiet_hours_enabled,omitempty"`
	QuietHoursStart         *string         `json:"quiet_hours_start,omitempty"` // HH:MM format
	QuietHoursEnd           *string         `json:"quiet_hours_end,omitempty"`
	QuietHoursTimezone      *string         `json:"quiet_hours_timezone,omitempty"`
	MaxNotificationsPerHour *int            `json:"max_notifications_per_hour,omitempty"`
	MaxNotificationsPerDay  *int            `json:"max_notifications_per_day,omitempty"`
}

type RegisterDeviceRequest struct {
	Token    string `json:"token" validate:"required"`
	Platform string `json:"platform" validate:"required,oneof=ios android web"`
}

type NotificationListResponse struct {
	Notifications []*Notification `json:"notifications"`
	UnreadCount   int             `json:"unread_count"`
	TotalCount    int             `json:"total_count"`
	Page          int             `json:"page"`
	PageSize      int             `json:"page_size"`
}