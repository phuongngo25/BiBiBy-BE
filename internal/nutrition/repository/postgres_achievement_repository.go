package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"nutrix-backend/internal/domain"
)

type postgresAchievementRepository struct {
	db *gorm.DB
}

// NewPostgresAchievementRepository creates a new achievement repository linked to the GORM DB.
func NewPostgresAchievementRepository(db *gorm.DB) domain.AchievementRepository {
	return &postgresAchievementRepository{db: db}
}

// Unlock records an achievement as unlocked for a user.
// Uses OnConflict to handle duplicate unlocks gracefully (No-op if already exists).
func (r *postgresAchievementRepository) Unlock(ctx context.Context, userID uuid.UUID, achievementID domain.AchievementID) error {
	achievement := domain.UserAchievement{
		UserID:        userID,
		AchievementID: achievementID,
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		DoNothing: true,
	}).Create(&achievement).Error
}

// IsUnlocked checks if a user has already unlocked a specific achievement.
func (r *postgresAchievementRepository) IsUnlocked(ctx context.Context, userID uuid.UUID, achievementID domain.AchievementID) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.UserAchievement{}).
		Where("user_id = ? AND achievement_id = ?", userID, achievementID).
		Count(&count).Error

	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUnlocked retrieves all user achievement records for a user.
func (r *postgresAchievementRepository) GetUnlocked(ctx context.Context, userID uuid.UUID) ([]domain.UserAchievement, error) {
	var achievements []domain.UserAchievement
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("unlocked_at desc").
		Find(&achievements).Error
	if err != nil {
		return nil, err
	}
	return achievements, nil
}
