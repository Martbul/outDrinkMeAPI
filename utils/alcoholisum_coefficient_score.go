package utils

import "math"

// CalculateAlcoholismScore calculates the raw coefficient based on usage data.
// Formula: (Streak^2 * 0.3) + (TotalDays * 0.05) + (Achievements * 1)
func CalculateAlcoholismScore(currentStreak, totalDays, achievementsCount int) float64 {
	streakScore := math.Pow(float64(currentStreak), 2) * 0.3
	daysScore := float64(totalDays) * 0.05
	achievementScore := float64(achievementsCount) * 1.0

	raw := streakScore + daysScore + achievementScore

	normilizedCoeff := GetNormalizedScore(raw)

	return float64(normilizedCoeff)
}

// GetNormalizedScore transforms the raw coefficient into a 0-100 scale using an inverse exponential curve.
// Logic matches: 100 * (1 - e^(-0.05 * raw))
func GetNormalizedScore(rawCoef float64) int {
	// If rawCoef is 0 (new user with no drinking data), return 0
	if rawCoef <= 0 {
		return 0
	}

	// Transform using inverse exponential
	// k = 0.05 controls growth rate
	k := 0.05
	transformedScore := 100 * (1 - math.Exp(-k*rawCoef))

	// Round to nearest integer
	coef := int(math.Round(transformedScore))

	// Normalize to 0-100 range
	if coef < 0 {
		return 0
	}
	if coef > 100 {
		return 100
	}

	return coef
}
