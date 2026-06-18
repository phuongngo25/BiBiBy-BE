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
	ID               string `json:"id"                 gorm:"primaryKey"`
	AscendID         string `json:"ascend_id"          gorm:"uniqueIndex;not null"`
	Name             string `json:"name"               gorm:"not null"`
	TargetMuscle     string `json:"target_muscle"`
	BodyPart         string `json:"body_part"`
	Equipment        string `json:"equipment"`
	GifUrl           string `json:"gif_url"`
	VideoUrl         string `json:"video_url"`
	Instructions     string `json:"instructions"`
	MuscleHeatmapUrl string `json:"muscle_heatmap_url"`
	// Legacy array fields — still used by ExerciseLog and migration
	BodyParts     pq.StringArray `json:"body_parts"     gorm:"type:text[]"`
	Equipments    pq.StringArray `json:"equipments"     gorm:"type:text[]"`
	TargetMuscles pq.StringArray `json:"target_muscles" gorm:"type:text[]"`
}

// MetActivity represents static exercises with predefined MET values (e.g. running, cycling).
type MetActivity struct {
	ID           string  `json:"id"             gorm:"primaryKey"`
	ActivityName string  `json:"activity_name"  gorm:"not null;uniqueIndex"`
	MetValue     float64 `json:"met_value"      gorm:"not null"`
	Category     string  `json:"category"`
}

// WorkoutLog records a single workout session performed by a user.
// ExerciseID is stored as a plain string — DO NOT add a foreignKey tag here
// because the ID originates from the external RapidAPI AscendAPI and is
// NOT guaranteed to exist in our local exercises table.
type WorkoutLog struct {
	ID              uint      `json:"id"               gorm:"primaryKey;autoIncrement"`
	UserID          uuid.UUID `json:"user_id"          gorm:"type:uuid;index;not null"`
	ExerciseID      string    `json:"exercise_id"      gorm:"type:varchar(100);not null"` // plain string, no FK
	ExerciseName    string    `json:"exercise_name"    gorm:"not null"`
	DurationMinutes int       `json:"duration_minutes" gorm:"not null"`
	CaloriesBurned  float64   `json:"calories_burned"  gorm:"not null"`
	LoggedAt        time.Time `json:"logged_at"        gorm:"autoCreateTime"`
}

// LogWorkoutRequest is the input DTO for the POST /workouts/log endpoint.
//
// MetValue is optional. When supplied (> 0) it overrides the default MET so
// activities with a known MET (e.g. the 2011 Compendium "Other Activities":
// bicycling, calisthenics, …) burn the correct number of calories. Clients
// that don't know the MET (the gym/resistance flow) omit it and the usecase
// falls back to the default MET.
type LogWorkoutRequest struct {
	ExerciseID      string  `json:"exercise_id"      binding:"required"`
	ExerciseName    string  `json:"exercise_name"    binding:"required"`
	DurationMinutes int     `json:"duration_minutes" binding:"required,gt=0"`
	MetValue        float64 `json:"met_value"`
}

// WorkoutRepository defines the data access boundary for the workout module.
type WorkoutRepository interface {
	// GetExercisesByMuscle returns cached exercises for a target muscle group.
	GetExercisesByMuscle(ctx context.Context, muscle string) ([]Exercise, error)
	// UpsertExercises batch-inserts exercises, ignoring duplicates by AscendID.
	UpsertExercises(ctx context.Context, exercises []Exercise) error
	// CountByMuscle returns how many exercises are cached for a muscle group.
	CountByMuscle(ctx context.Context, muscle string) (int64, error)
	// LogWorkout persists a WorkoutLog record.
	LogWorkout(ctx context.Context, log *WorkoutLog) error
	// GetDailyBurnedCalories aggregates all calories burned by a user on a given date.
	GetDailyBurnedCalories(ctx context.Context, userID uuid.UUID, date time.Time) (float64, error)
}

// WorkoutUseCase defines the core business logic for the workout module.
// In this iteration, it proxies directly to the RapidAPI AscendAPI endpoints.
type WorkoutUseCase interface {
	GetExercisesByBodyParts(ctx context.Context, bodyParts string) ([]ExerciseListItem, error)
	GetExerciseByID(ctx context.Context, id string) (*ExerciseDetail, error)
	// LogWorkout calculates calories burned and persists the workout session.
	LogWorkout(ctx context.Context, userID uuid.UUID, req *LogWorkoutRequest) (*WorkoutLog, error)
	// GetMuscleHeatmap proxies the muscle-activation heatmap image for a muscle,
	// returning the raw image bytes and Content-Type (keeps the API key server-side).
	GetMuscleHeatmap(ctx context.Context, muscle string) ([]byte, string, error)
	// GetExerciseAsset proxies an exercise image/GIF from the public CDN
	// (same-origin) so Flutter web can load it despite the CDN missing CORS.
	// token is the base64url-encoded CDN URL.
	GetExerciseAsset(ctx context.Context, token string) ([]byte, string, error)
}

// ─── NEW PROXY DTOs ──────────────────────────────────────────────────────────

type ExerciseListItem struct {
	ExerciseID    string   `json:"exercise_id"`
	Name          string   `json:"name"`
	ImageUrl      string   `json:"image_url"`
	Equipment     string   `json:"equipment,omitempty"`
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
