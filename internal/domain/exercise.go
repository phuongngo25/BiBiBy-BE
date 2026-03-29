package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Exercise maps to external API data (like RapidAPI).
type Exercise struct {
	ID            string         `json:"id"             gorm:"primaryKey"`
	Name          string         `json:"name"           gorm:"not null"`
	ExerciseType  string         `json:"exercise_type"`
	ImageURL      string         `json:"image_url"`
	BodyParts     pq.StringArray `json:"body_parts"     gorm:"type:text[]"`
	Equipments    pq.StringArray `json:"equipments"     gorm:"type:text[]"`
	TargetMuscles pq.StringArray `json:"target_muscles" gorm:"type:text[]"`
}

// ExerciseLog represents a user logging a specific workout.
type ExerciseLog struct {
	ID              uuid.UUID `json:"id"               gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID          uuid.UUID `json:"user_id"          gorm:"index;not null;constraint:OnDelete:CASCADE;"`
	ExerciseID      string    `json:"exercise_id"      gorm:"index;not null"`
	PerformedDate   time.Time `json:"performed_date"   gorm:"type:date;not null"`
	DurationMinutes int       `json:"duration_minutes" gorm:"not null"`
	CaloriesBurned  float64   `json:"calories_burned"`
	LoggedAt        time.Time `json:"logged_at"        gorm:"autoCreateTime"`

	Exercise        Exercise  `json:"exercise"         gorm:"foreignKey:ExerciseID"`
}
