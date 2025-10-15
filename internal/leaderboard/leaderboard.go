package leaderboard

import "github.com/google/uuid"

type LeaderboardEntry struct {
	UserID          uuid.UUID `json:"user_id" db:"user_id"`
	Username        string    `json:"username" db:"username"`
	DisplayName     *string   `json:"display_name" db:"display_name"`
	AvatarURL       *string   `json:"avatar_url" db:"avatar_url"`
	DaysThisWeek    int       `json:"days_this_week" db:"days_this_week"`
	Rank            int       `json:"rank" db:"rank"`
	CurrentStreak   int       `json:"current_streak" db:"current_streak"`
}

type Leaderboard struct {
	Entries      []*LeaderboardEntry `json:"entries"`
	UserPosition *LeaderboardEntry   `json:"user_position"`
	TotalUsers   int                 `json:"total_users"`
}