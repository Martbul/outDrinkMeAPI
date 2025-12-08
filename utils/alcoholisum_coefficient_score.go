package utils

import "math"

// Formula: (Streak^2 * 0.6) + (TotalDays * 0.05) + (Achievements * 1)
func CalculateAlcoholismScore(currentStreak, totalDays, achievementsCount int) float64 {
	streakScore := math.Pow(float64(currentStreak), 2) * 0.6
	daysScore := float64(totalDays) * 0.05
	achievementScore := float64(achievementsCount) * 1.0

	raw := streakScore + daysScore + achievementScore

	normilizedCoeff := GetNormalizedScore(raw)

	return float64(normilizedCoeff)
}

// Logic matches: 100 * (1 - e^(-0.05 * raw))
func GetNormalizedScore(rawCoef float64) int {
	// If rawCoef is 0 or negative, we default to the minimum score of 1.
	if rawCoef <= 0 {
		return 1
	}

	// Transform using inverse exponential
	// k = 0.05 controls growth rate
	k := 0.05
	transformedScore := 100 * (1 - math.Exp(-k*rawCoef))

	// Round to nearest integer
	coef := int(math.Round(transformedScore))

	// Normalize to 1-100 range
	// We check specifically if the result is less than 1 (which handles rounding down of small numbers)
	if coef < 1 {
		return 1
	}
	if coef > 100 {
		return 100
	}

	return coef
}