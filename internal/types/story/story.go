package story

import (
	"time"
	"github.com/google/uuid"
)

type Story struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	VideoUrl      string    `json:"video_url"`
	VideoWidth    uint      `json:"video_width"`
	VideoHeight   uint      `json:"video_height"`
	VideoDuration uint      `json:"video_duration"`
	RelateCount   int       `json:"relate_count"`
	HasRelated    bool      `json:"has_related"` // If the current user liked it
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type CreateStoryRequest struct {
	VideoURL      string   `json:"videoUrl"`
	Width         int      `json:"width"`
	Height        int      `json:"height"`
	Duration      float64  `json:"duration"`
	TaggedBuddies []string `json:"taggedBuddies"` 
}

type RelateStoryRequest struct {
	StoryID string `json:"storyId"`
	Action  string `json:"action"` 
}

type DeleteStoryRequest struct {
	StoryID string `json:"storyId"`
}