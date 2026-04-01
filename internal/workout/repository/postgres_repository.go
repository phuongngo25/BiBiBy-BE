package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nutrix-backend/internal/domain"
)

// postgresWorkoutRepository is the GORM-backed implementation of domain.WorkoutRepository.
type postgresWorkoutRepository struct {
	db *gorm.DB
}

// NewPostgresWorkoutRepository constructs a new repository instance.
// The caller (main.go) is responsible for running AutoMigrate for domain.Exercise.
func NewPostgresWorkoutRepository(db *gorm.DB) domain.WorkoutRepository {
	return &postgresWorkoutRepository{db: db}
}

// GetExercisesByMuscle returns all cached exercises for a target muscle.
// Case-insensitive match via ILIKE so "biceps" and "Biceps" both hit.
func (r *postgresWorkoutRepository) GetExercisesByMuscle(ctx context.Context, muscle string) ([]domain.Exercise, error) {
	var exercises []domain.Exercise
	result := r.db.WithContext(ctx).
		Where("LOWER(target_muscle) = LOWER(?)", muscle).
		Find(&exercises)
	return exercises, result.Error
}

// CountByMuscle returns how many exercises are cached for a given muscle group.
func (r *postgresWorkoutRepository) CountByMuscle(ctx context.Context, muscle string) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Model(&domain.Exercise{}).
		Where("LOWER(target_muscle) = LOWER(?)", muscle).
		Count(&count)
	return count, result.Error
}

// UpsertExercises batch-inserts exercises into the DB.
// clause.OnConflict{DoNothing: true} means: if a row with the same `ascend_id`
// already exists, skip it silently — never overwrite our cached data.
func (r *postgresWorkoutRepository) UpsertExercises(ctx context.Context, exercises []domain.Exercise) error {
	if len(exercises) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ascend_id"}},
			DoNothing: true,
		}).
		CreateInBatches(exercises, 50)
	return result.Error
}
