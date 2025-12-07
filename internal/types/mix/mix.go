package mix

import (
	"outDrinkMeAPI/internal/types/achievement"
	"outDrinkMeAPI/internal/types/stats"
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


type FriendDiscoveryDisplayProfileResponse struct {
	User         *user.User                                `json:"user"`
	Stats        *stats.UserStats                     `json:"stats"`
	Achievements []*achievement.AchievementWithStatus `json:"achievements"`
	MixPosts     []DailyDrinkingPost                  `json:"mix_posts"`
	IsFriend     bool                                 `json:"is_friend"`
}
