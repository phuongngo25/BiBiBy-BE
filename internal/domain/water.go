package domain

import (
	"time"

	"github.com/google/uuid"
)

// WaterLog represents an append-only hydration event.
type WaterLog struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;index:idx_water_user_date,priority:1;not null"`
	AmountMl  int       `gorm:"not null"`
	Source    string    `gorm:"type:varchar(50)"`
	CreatedAt time.Time `gorm:"index:idx_water_user_date,priority:2;autoCreateTime"`
}

// LogWaterRequest is the payload to track water intake.
type LogWaterRequest struct {
	AmountMl int    `json:"amount_ml" binding:"required,gt=0,lte=2000"`
	Source   string `json:"source"`
}

// LogWaterResponse is the DTO returned when water is tracked successfully.
type LogWaterResponse struct {
	ID        uuid.UUID `json:"id"`
	AmountMl  int       `json:"amount_ml"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

// CalculateDailyWaterTarget is a pure function that calculates the daily water target (in ml)
// for a user based on weight, gender, and activity level.
func CalculateDailyWaterTarget(user User) int {
	// Base = WeightKg * 30
	base := int(user.WeightKg * 30)

	// Gender modifiers
	if user.Gender == "male" {
		base += 250
	}

	// Activity modifiers
	switch user.ActivityLevel {
	case ActivitySedentary:
		base += 0
	case ActivityLowActive:
		base += 350
	case ActivityActive:
		base += 700
	case ActivityVeryActive:
		base += 1000
	}

	// Clamp final value between 1500 ml and 5000 ml
	if base < 1500 {
		return 1500
	}
	if base > 5000 {
		return 5000
	}
	return base
}
