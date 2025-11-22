package mix

import (
	"outDrinkMeAPI/internal/types/user"
	"time"
)

type DailyDrinkingPost struct {
	ID               string
	UserID           string
	UserImageURL     *string
	Date             time.Time
	DrankToday       bool
	LoggedAt         time.Time
	ImageURL         *string
	LocationText     *string
	MentionedBuddies []user.User
	SourceType       string
}

type VideoPost struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	UserImageUrl string    `json:"user_image_url"`
	VideoUrl     string    `json:"video_url"`
	Caption      string    `json:"caption"`
	Chips        int       `json:"chips"`
	Duration     int       `json:"duration"`
	CreatedAt    time.Time `json:"created_at"`
}