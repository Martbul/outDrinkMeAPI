package streak

import (
	"time"

	"github.com/google/uuid"
)

type Streak struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	CurrentStreak int        `json:"current_streak" db:"current_streak"`
	LongestStreak int        `json:"longest_streak" db:"longest_streak"`
	LastDrinkDate *time.Time `json:"last_drink_date" db:"last_drink_date"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}