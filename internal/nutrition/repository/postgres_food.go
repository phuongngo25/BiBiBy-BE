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

type postgresNutritionRepository struct {
	db *gorm.DB
}

// NewPostgresNutritionRepository creates a repository hooked up to the GORM DB instance.
func NewPostgresNutritionRepository(db *gorm.DB) domain.NutritionRepository {
	return &postgresNutritionRepository{db: db}
}

// GetFoodByID fetches a single Food by its ID.
func (r *postgresNutritionRepository) GetFoodByID(ctx context.Context, id uuid.UUID) (*domain.Food, error) {
	var food domain.Food
	err := r.db.WithContext(ctx).First(&food, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrFoodNotFound
		}
		return nil, domain.ErrInternalServerError
	}
	return &food, nil
}

// SearchFoods handles fetching foods matching the given keyword (case-insensitive).
func (r *postgresNutritionRepository) SearchFoods(ctx context.Context, keyword string) ([]domain.Food, error) {
	var foods []domain.Food
	err := r.db.WithContext(ctx).Where("name ILIKE ?", "%"+keyword+"%").Find(&foods).Error
	if err != nil {
		return nil, domain.ErrInternalServerError
	}
	return foods, nil
}

// SearchFoodsByNutrients queries the DB using strict numeric bounds.
// Zero values are treated as "no constraint" for that field.
func (r *postgresNutritionRepository) SearchFoodsByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]domain.Food, error) {
	var foods []domain.Food
	q := r.db.WithContext(ctx).Model(&domain.Food{})

	if minProtein > 0 {
		q = q.Where("protein_per_100g >= ?", minProtein)
	}
	if maxFat > 0 {
		q = q.Where("fat_per_100g <= ?", maxFat)
	}
	if minCalories > 0 {
		q = q.Where("calories_per_100g >= ?", minCalories)
	}
	if maxCalories > 0 {
		q = q.Where("calories_per_100g <= ?", maxCalories)
	}

	if err := q.Find(&foods).Error; err != nil {
		return nil, domain.ErrInternalServerError
	}
	return foods, nil
}

// GetRandomFoods returns a specified number of randomly ordered foods.
func (r *postgresNutritionRepository) GetRandomFoods(ctx context.Context, limit int) ([]domain.Food, error) {
	var foods []domain.Food
	err := r.db.WithContext(ctx).Order("RANDOM()").Limit(limit).Find(&foods).Error
	if err != nil {
		return nil, domain.ErrInternalServerError
	}
	return foods, nil
}

// CreateFood inserts a new custom Food record into the database.
func (r *postgresNutritionRepository) CreateFood(ctx context.Context, food *domain.Food) error {
	return r.db.WithContext(ctx).Create(food).Error
}

// UpsertFoods bulk-inserts Spoonacular-sourced foods.
// On spoonacular_id conflict, DoNothing skips redundant write-locks.
func (r *postgresNutritionRepository) UpsertFoods(ctx context.Context, foods []domain.Food) error {
	if len(foods) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "spoonacular_id"}},
			DoNothing: true,
		}).
		Create(&foods).Error
}

// LogMeal inserts a MealLog record into the database.
func (r *postgresNutritionRepository) LogMeal(ctx context.Context, log *domain.MealLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetDailyLogs fetches all meals a user consumed on a specific calendar day.
func (r *postgresNutritionRepository) GetDailyLogs(ctx context.Context, userID uuid.UUID, date time.Time) ([]domain.MealLog, error) {
	var logs []domain.MealLog

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	err := r.db.WithContext(ctx).
		Preload("Food").
		Where("user_id = ? AND consumed_date >= ? AND consumed_date < ?", userID, startOfDay, endOfDay).
		Find(&logs).Error

	if err != nil {
		return nil, domain.ErrInternalServerError
	}
	return logs, nil
}

// WithTransaction runs the enclosed lambda functionally inside a strict Database Transaction.
func (r *postgresNutritionRepository) WithTransaction(ctx context.Context, fn func(repo domain.NutritionRepository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txRepo := &postgresNutritionRepository{db: tx}
		return fn(txRepo)
	})
}

