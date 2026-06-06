package domain

// ActivityLevel represents the user's daily physical activity multiplier.
type ActivityLevel string

const (
	ActivitySedentary  ActivityLevel = "sedentary"
	ActivityLowActive  ActivityLevel = "low_active"
	ActivityActive     ActivityLevel = "active"
	ActivityVeryActive ActivityLevel = "very_active"
)

// GoalType represents the user's target calorie direction.
type GoalType string

const (
	GoalLoseWeight GoalType = "lose_weight"
	GoalMaintain   GoalType = "maintain"
	GoalGainWeight GoalType = "gain_weight"
)
