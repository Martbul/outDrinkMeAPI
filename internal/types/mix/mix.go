package mix

import (
	"outDrinkMeAPI/internal/types/achievement"
	"outDrinkMeAPI/internal/types/canvas"
	"outDrinkMeAPI/internal/types/stats"
	"outDrinkMeAPI/internal/types/store"
	"outDrinkMeAPI/internal/types/user"
	"time"
)

type DailyDrinkingPost struct {
	ID               string              `json:"id"`
	UserID           string              `json:"user_id"`
	Username         string              `json:"username"`
	UserImageURL     *string             `json:"user_image_url"`
	Date             time.Time           `json:"date"`
	DrankToday       bool                `json:"drank_today"`
	LoggedAt         time.Time           `json:"logged_at"`
	ImageURL         *string             `json:"image_url"`
	LocationText     *string             `json:"location_text"`
	Latitude         *float64            `json:"latitude"`
	Longitude        *float64            `json:"longitude"`
	Alcohols         []string            `json:"alcohol"`
	MentionedBuddies []user.User         `json:"mentioned_buddies"`
	SourceType       string              `json:"source_type"`
	Reactions        []canvas.CanvasItem `json:"reactions"`

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
	User         *user.User                           `json:"user"`
	Stats        *stats.UserStats                     `json:"stats"`
	Achievements []*achievement.AchievementWithStatus `json:"achievements"`
	MixPosts     []DailyDrinkingPost                  `json:"mix_posts"`
	IsFriend     bool                                 `json:"is_friend"`
	Inventory    map[string][]*store.InventoryItem    `json:"inventory"` // Added this field
}
