package utils

import "math"

func CalculateAlcoholismScore(currentStreak, totalDays, achievementsCount int) float64 {
	streakScore := math.Pow(float64(currentStreak), 2) * 0.3
	daysScore := float64(totalDays) * 0.05
	achievementScore := float64(achievementsCount) * 1.0

	return streakScore + daysScore + achievementScore
}