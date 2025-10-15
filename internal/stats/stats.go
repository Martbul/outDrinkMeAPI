package stats

type DaysStat struct {
	Period    string `json:"period"` // "week", "month", "year", "all_time"
	DaysDrank int    `json:"days_drank" db:"days_drank"`
	TotalDays int    `json:"total_days"`
}

type UserStats struct {
	TodayStatus       bool `json:"today_status"`
	DaysThisWeek      int  `json:"days_this_week"`
	DaysThisMonth     int  `json:"days_this_month"`
	DaysThisYear      int  `json:"days_this_year"`
	TotalDaysDrank    int  `json:"total_days_drank"`
	CurrentStreak     int  `json:"current_streak"`
	LongestStreak     int  `json:"longest_streak"`
	TotalWeeksWon     int  `json:"total_weeks_won"`
	AchievementsCount int  `json:"achievements_count"`
	FriendsCount      int  `json:"friends_count"`
	Rank              int  `json:"rank"`
}