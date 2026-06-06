package service

import (
	"math"

	"nutrix-backend/internal/domain"
)

// GoalStrategy defines how nutritional and hydration targets are derived from a user's health profile.
type GoalStrategy interface {
	CalculateTargets(user *domain.User) (targetCalories int, targetWater int)
}

type goalStrategyV1 struct{}

// NewGoalStrategyV1 creates a new instance of GoalStrategy (version "v1").
func NewGoalStrategyV1() GoalStrategy {
	return &goalStrategyV1{}
}

// CalculateTargets computes target daily calories and water.
// It prioritizes the manual WeeklyCalorieBudget override, falling back to a GoalType-based shift from TDEE.
func (s *goalStrategyV1) CalculateTargets(user *domain.User) (int, int) {
	var targetCalories int

	// 1. Check for manual override: WeeklyCalorieBudget / 7
	if user.WeeklyCalorieBudget > 0 {
		targetCalories = int(math.Round(user.WeeklyCalorieBudget / 7.0))
	} else {
		// 2. Goal-based shifts applied to TDEE
		tdee := int(math.Round(user.TDEE))
		if tdee <= 0 {
			// Fallback base TDEE if profile data is missing/invalid
			tdee = 2000
		}

		switch user.GoalType {
		case domain.GoalLoseWeight:
			targetCalories = tdee - 500
		case domain.GoalGainWeight:
			targetCalories = tdee + 500
		case domain.GoalMaintain:
			targetCalories = tdee
		default:
			targetCalories = tdee
		}
	}

	// Enforce healthy baseline limit (1200 kcal) for calculated target calories
	if targetCalories < 1200 {
		targetCalories = 1200
	}

	// Calculate target water in ml using standard domain formula
	targetWater := domain.CalculateDailyWaterTarget(*user)

	return targetCalories, targetWater
}
