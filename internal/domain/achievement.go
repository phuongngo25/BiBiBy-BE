package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AchievementID is a unique identifier for an achievement.
type AchievementID string

type AchievementCategory string

const (
	CategoryStreak    AchievementCategory = "streak"
	CategoryMilestone AchievementCategory = "milestone"
)

type GamificationTrigger string

const (
	TriggerStreakUpdated GamificationTrigger = "streak_updated"
	TriggerDailyGoal     GamificationTrigger = "daily_goal"
)

const (
	AchievementFirstGoalHit AchievementID = "first_goal_hit"
	AchievementStreak7      AchievementID = "streak_7"
	AchievementStreak30     AchievementID = "streak_30"
	AchievementPerfectDay   AchievementID = "perfect_day"
)

// UserAchievement represents an achievement unlocked by a user.
type UserAchievement struct {
	UserID        uuid.UUID     `gorm:"type:uuid;primaryKey" json:"user_id"`
	AchievementID AchievementID `gorm:"type:varchar(50);primaryKey" json:"achievement_id"`
	UnlockedAt    time.Time     `gorm:"autoCreateTime" json:"unlocked_at"`
}

// TableName overrides the table name to match database conventions.
func (UserAchievement) TableName() string {
	return "user_achievements"
}

// AchievementDefinition defines the metadata for an achievement.
type AchievementDefinition struct {
	ID          AchievementID       `json:"id"`
	Category    AchievementCategory `json:"category"`
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Icon        string              `json:"icon"`
	SortOrder   int                 `json:"sort_order"`
	Hidden      bool                `json:"hidden"`
}

// AchievementRepository defines database capabilities for user achievements.
type AchievementRepository interface {
	Unlock(ctx context.Context, userID uuid.UUID, achievementID AchievementID) error
	IsUnlocked(ctx context.Context, userID uuid.UUID, achievementID AchievementID) (bool, error)
	GetUnlocked(ctx context.Context, userID uuid.UUID) ([]UserAchievement, error)
}

// UnlockedAchievementDTO maps the unlocked metadata to be returned to the client.
type UnlockedAchievementDTO struct {
	AchievementID AchievementID `json:"achievement_id"`
	UnlockedAt    time.Time     `json:"unlocked_at"`
}

// AchievementResponse is the DTO for achievements.
type AchievementResponse struct {
	Definitions []AchievementDefinition  `json:"definitions"`
	Unlocked    []UnlockedAchievementDTO `json:"unlocked"`
}

// GamificationUseCase defines business logic for achievements and rewards.
type GamificationUseCase interface {
	GetAchievements(ctx context.Context, userID uuid.UUID) (*AchievementResponse, error)
}
