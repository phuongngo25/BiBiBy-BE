package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"nutrix-backend/internal/domain"
)

type postgresDRIRepository struct {
	db *gorm.DB
}

// NewPostgresDRIRepository creates a new PostgreSQL-backed DRIRepository.
func NewPostgresDRIRepository(db *gorm.DB) domain.DRIRepository {
	return &postgresDRIRepository{db: db}
}

// GetByDemographic fetches the DRI row that matches the given life stage group and age range.
func (r *postgresDRIRepository) GetByDemographic(ctx context.Context, lifeStage string, ageRange string) (*domain.DRI, error) {
	var dri domain.DRI
	err := r.db.WithContext(ctx).
		Where("life_stage_group = ? AND age_range = ?", lifeStage, ageRange).
		First(&dri).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrDRINotFound
		}
		return nil, err
	}
	return &dri, nil
}
