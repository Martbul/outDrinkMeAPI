package leaderboard

import "github.com/google/uuid"

type LeaderboardEntry struct {
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	Username        string    `json:"username" db:"username"`
	ImageURL       *string   `json:"image_url" db:"image_url"`
	DaysThisWeek    int       `json:"days_this_week" db:"days_this_week"`
	Rank            int       `json:"rank" db:"rank"`
	CurrentStreak   int       `json:"current_streak" db:"current_streak"`
}

type Leaderboard struct {
	Entries      []*LeaderboardEntry `json:"entries"`
	UserPosition *LeaderboardEntry   `json:"user_position"`
	TotalUsers   int                 `json:"total_users"`
}