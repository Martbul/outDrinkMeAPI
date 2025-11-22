package challenge

import (
	"time"

	"github.com/google/uuid"
)

type GoalType string

const (
	GoalDailyStreak  GoalType = "daily_streak"
	GoalWeeklyDays   GoalType = "weekly_days"
	GoalPerfectWeek  GoalType = "perfect_week"
)

type Challenge struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	StartDate   time.Time `json:"start_date" db:"start_date"`
	EndDate     time.Time `json:"end_date" db:"end_date"`
	GoalType    GoalType  `json:"goal_type" db:"goal_type"`
	GoalValue   int       `json:"goal_value" db:"goal_value"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type UserChallenge struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	ChallengeID uuid.UUID  `json:"challenge_id" db:"challenge_id"`
	Progress    int        `json:"progress" db:"progress"`
	Completed   bool       `json:"completed" db:"completed"`
	CompletedAt *time.Time `json:"completed_at" db:"completed_at"`
	JoinedAt    time.Time  `json:"joined_at" db:"joined_at"`
}
