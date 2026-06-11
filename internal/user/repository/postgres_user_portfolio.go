package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"nutrix-backend/internal/domain"
)

type postgresUserPortfolioRepository struct {
	db *gorm.DB
}

func NewPostgresUserPortfolioRepository(db *gorm.DB) domain.UserPortfolioRepository {
	return &postgresUserPortfolioRepository{db: db}
}

func (r *postgresUserPortfolioRepository) GetPortfolio(ctx context.Context, userID uuid.UUID) (*domain.UserPortfolio, error) {
	var portfolio domain.UserPortfolio
	err := r.db.WithContext(ctx).First(&portfolio, "user_id = ?", userID).Error
	if err == nil {
		return &portfolio, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &domain.UserPortfolio{UserID: userID}, nil
	}
	return nil, err
}

func (r *postgresUserPortfolioRepository) UpsertPortfolio(ctx context.Context, portfolio *domain.UserPortfolio) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"preferred_cuisines",
				"disliked_ingredients",
				"excluded_ingredients",
				"meal_schedule",
				"daily_water_target_ml",
				"calorie_target_override",
				"macro_split_override",
				"notes",
				"updated_at",
			}),
		}).
		Create(portfolio).Error
}
