package achievement

import (
	"time"

	"github.com/google/uuid"
)

type CriteriaType string

const (
	CriteriaStreak      CriteriaType = "streak"
	CriteriaTotalDays   CriteriaType = "total_days"
	CriteriaWeeksWon    CriteriaType = "weeks_won"
	CriteriaFriends     CriteriaType = "friends"
	CriteriaPerfectWeek CriteriaType = "perfect_week"
)

type Achievement struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	Name          string       `json:"name" db:"name"`
	Description   string       `json:"description" db:"description"`
	Icon          string       `json:"icon" db:"icon"`
	CriteriaType  CriteriaType `json:"criteria_type" db:"criteria_type"`
	CriteriaValue int          `json:"criteria_value" db:"criteria_value"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
}

type UserAchievement struct {
	ID            uuid.UUID `json:"id" db:"id"`
	UserID        uuid.UUID `json:"user_id" db:"user_id"`
	AchievementID uuid.UUID `json:"achievement_id" db:"achievement_id"`
	UnlockedAt    time.Time `json:"unlocked_at" db:"unlocked_at"`
}

type AchievementWithStatus struct {
	Achievement
	Unlocked   bool       `json:"unlocked"`
	UnlockedAt *time.Time `json:"unlocked_at,omitempty"`
}
