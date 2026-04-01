package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Exercise maps to the AscendAPI exercise data cached in our Postgres DB.
// AscendID has a unique index so we can safely upsert without duplicates.
type Exercise struct {
	ID               string         `json:"id"                 gorm:"primaryKey"`
	AscendID         string         `json:"ascend_id"          gorm:"uniqueIndex;not null"`
	Name             string         `json:"name"               gorm:"not null"`
	TargetMuscle     string         `json:"target_muscle"`
	BodyPart         string         `json:"body_part"`
	Equipment        string         `json:"equipment"`
	GifUrl           string         `json:"gif_url"`
	VideoUrl         string         `json:"video_url"`
	Instructions     string         `json:"instructions"`
	MuscleHeatmapUrl string         `json:"muscle_heatmap_url"`
	// Legacy array fields — still used by ExerciseLog and migration
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

	Exercise Exercise `json:"exercise" gorm:"foreignKey:ExerciseID"`
}

// WorkoutRepository defines the data access boundary for the workout module.
type WorkoutRepository interface {
	// GetExercisesByMuscle returns cached exercises for a target muscle group.
	GetExercisesByMuscle(ctx context.Context, muscle string) ([]Exercise, error)
	// UpsertExercises batch-inserts exercises, ignoring duplicates by AscendID.
	UpsertExercises(ctx context.Context, exercises []Exercise) error
	// CountByMuscle returns how many exercises are cached for a muscle group.
	CountByMuscle(ctx context.Context, muscle string) (int64, error)
}

// WorkoutUseCase defines the core business logic for the workout module.
// In this iteration, it proxies directly to the RapidAPI AscendAPI endpoints.
type WorkoutUseCase interface {
	GetExercisesByBodyParts(ctx context.Context, bodyParts string) ([]ExerciseListItem, error)
	GetExerciseByID(ctx context.Context, id string) (*ExerciseDetail, error)
}

// ─── NEW PROXY DTOs ──────────────────────────────────────────────────────────

type ExerciseListItem struct {
	ExerciseID    string   `json:"exercise_id"`
	Name          string   `json:"name"`
	ImageUrl      string   `json:"image_url"`
	BodyParts     []string `json:"body_parts"`
	TargetMuscles []string `json:"target_muscles"`
}

type GifUrls struct {
	P720  string `json:"720p"`
	P1080 string `json:"1080p"`
}

type ExerciseDetail struct {
	ExerciseID       string   `json:"exercise_id"`
	Name             string   `json:"name"`
	GifUrls          GifUrls  `json:"gif_urls"`
	Instructions     []string `json:"instructions"`
	Equipments       []string `json:"equipments"`
	MuscleHeatmapUrl string   `json:"muscle_heatmap_url"`
}

