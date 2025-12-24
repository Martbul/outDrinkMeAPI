package story

import (
	"time"
	"github.com/google/uuid"
)

type Story struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"userId"`
	Username     string    `json:"username"`
	UserImage    string    `json:"userImage"`
	VideoURL     string    `json:"videoUrl"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Duration     float64   `json:"duration"`
	CreatedAt    time.Time `json:"createdAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
	IsSeen       bool      `json:"isSeen"` 
	TaggedUsers  []string  `json:"taggedUsers"` 
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