package domain

import (
	"time"

	"github.com/google/uuid"
)

// DailyHealthSnapshot represents a frozen daily record of a user's health metrics and nutritional goals.
// It serves as the single source of truth for daily logs, analytics, and streak tracking.
type DailyHealthSnapshot struct {
	ID                  uuid.UUID     `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID              uuid.UUID     `gorm:"type:uuid;uniqueIndex:idx_user_date;not null" json:"user_id"`
	SnapshotDate        time.Time     `gorm:"type:date;uniqueIndex:idx_user_date;not null" json:"snapshot_date"`
	
	// Physiological state at the time
	WeightKg            float64       `gorm:"not null" json:"weight_kg"`
	ActivityLevel       ActivityLevel `gorm:"type:varchar(50);not null" json:"activity_level"`
	GoalType            GoalType      `gorm:"type:varchar(50);not null" json:"goal_type"`
	BMR                 int           `gorm:"not null" json:"bmr"`
	TDEE                int           `gorm:"not null" json:"tdee"`
	
	// Calculated targets based on the state above
	TargetCalories      int           `gorm:"not null" json:"target_calories"`
	TargetWater         int           `gorm:"not null" json:"target_water"`
	
	// Auditable logic version (e.g., "v1")
	GoalStrategyVersion string        `gorm:"type:varchar(20);not null;default:'v1'" json:"goal_strategy_version"`
	
	CreatedAt           time.Time     `gorm:"autoCreateTime" json:"created_at"`
}
