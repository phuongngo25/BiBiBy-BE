package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"nutrix-backend/internal/domain"
)

type postgresStreakRepository struct {
	db *gorm.DB
}

// NewPostgresStreakRepository creates a new streak repository linked to the GORM DB.
func NewPostgresStreakRepository(db *gorm.DB) domain.StreakRepository {
	return &postgresStreakRepository{db: db}
}

// GetStreak fetches the cached streak for the given user.
// Returns (nil, nil) if no streak record exists in the derived cache.
func (r *postgresStreakRepository) GetStreak(ctx context.Context, userID uuid.UUID) (*domain.UserStreak, error) {
	var streak domain.UserStreak
	err := r.db.WithContext(ctx).First(&streak, "user_id = ?", userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &streak, nil
}

// UpsertStreak performs an atomic, race-safe Postgres upsert on the derived cache table.
func (r *postgresStreakRepository) UpsertStreak(ctx context.Context, streak *domain.UserStreak) error {
	streak.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"current_streak", "longest_streak", "last_evaluated_date", "updated_at"}),
	}).Create(streak).Error
}
