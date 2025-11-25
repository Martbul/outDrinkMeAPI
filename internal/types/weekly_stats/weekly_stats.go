package weekly_stats

import (
	"time"

	"github.com/google/uuid"
)

type WeeklyStats struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	WeekStart  time.Time `json:"week_start" db:"week_start"`
	WeekEnd    time.Time `json:"week_end" db:"week_end"`
	DaysDrank  int       `json:"days_drank" db:"days_drank"`
	TotalDays  int       `json:"total_days" db:"total_days"`
	WinCount   int       `json:"win_count" db:"win_count"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}
