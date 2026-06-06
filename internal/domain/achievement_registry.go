package domain

// AchievementRegistry is the compile-time list of all available achievements.
var AchievementRegistry = []AchievementDefinition{
	{
		ID:          AchievementFirstGoalHit,
		Category:    CategoryMilestone,
		Title:       "First Goal Hit",
		Description: "You've successfully hit your first daily nutritional goal!",
		Icon:        "goal_reached",
		SortOrder:   1,
		Hidden:      false,
	},
	{
		ID:          AchievementStreak7,
		Category:    CategoryStreak,
		Title:       "7 Day Streak",
		Description: "Consistency is key! You've logged for 7 days in a row.",
		Icon:        "streak_7",
		SortOrder:   2,
		Hidden:      false,
	},
	{
		ID:          AchievementStreak30,
		Category:    CategoryStreak,
		Title:       "30 Day Streak",
		Description: "Monthly Master! You've maintained a 30-day streak.",
		Icon:        "streak_30",
		SortOrder:   3,
		Hidden:      false,
	},
	{
		ID:          AchievementPerfectDay,
		Category:    CategoryMilestone,
		Title:       "Perfect Day",
		Description: "All macros hit perfectly within targets!",
		Icon:        "perfect_day",
		SortOrder:   4,
		Hidden:      false,
	},
}
