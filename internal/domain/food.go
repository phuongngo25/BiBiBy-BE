package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Food represents a single measurable item in the database.
type Food struct {
	ID              uuid.UUID         `json:"id"                gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	SpoonacularID   *int              `json:"spoonacular_id"    gorm:"uniqueIndex"`
	Code            string            `json:"code"              gorm:"uniqueIndex"`
	Name            string            `json:"name"`
	NameVi          string            `json:"name_vi"`
	NameEn          string            `json:"name_en"`
	Category        string            `json:"category"`
	Source          string            `json:"source"            gorm:"default:'VFA'"`
	CaloriesPer100g float64           `json:"calories_per_100g" gorm:"column:calories_per_100g"`
	ProteinPer100g  float64           `json:"protein_per_100g"  gorm:"column:protein_per_100g"`
	CarbsPer100g    float64           `json:"carbs_per_100g"    gorm:"column:carbs_per_100g"`
	FatPer100g      float64           `json:"fat_per_100g"      gorm:"column:fat_per_100g"`
	ServingSize     string            `json:"serving_size"`
	Micronutrients  datatypes.JSONMap `json:"micronutrients"    gorm:"type:jsonb;default:'{}'"`
	IsVegan         bool              `json:"is_vegan"`
	IsVegetarian    bool              `json:"is_vegetarian"`
	IsGlutenFree    bool              `json:"is_gluten_free"`
	IsDairyFree     bool              `json:"is_dairy_free"`
	ImageURL        string            `json:"image_url"`
	IsVerified      bool              `json:"is_verified"       gorm:"default:true"`
	CreatorID       *uuid.UUID        `json:"creator_id"`
}

// MealLog (FoodLog) represents a user logging a specific food consumption.
type MealLog struct {
	ID               uuid.UUID `json:"id"                gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID           uuid.UUID `json:"user_id"           gorm:"index;not null;constraint:OnDelete:CASCADE;"`
	FoodID           uuid.UUID `json:"food_id"           gorm:"index;not null"`
	ConsumedDate     time.Time `json:"consumed_date"     gorm:"type:date;not null"`
	MealType         string    `json:"meal_type"`
	QuantityGrams    float64   `json:"quantity_grams"    gorm:"not null"`
	CaloriesConsumed float64   `json:"calories_consumed" gorm:"not null"`
	ProteinConsumed  float64   `json:"protein_consumed"  gorm:"not null"`
	FatConsumed      float64   `json:"fat_consumed"      gorm:"not null"`
	CarbsConsumed    float64   `json:"carbs_consumed"    gorm:"not null"`
	LoggedAt         time.Time `json:"logged_at"         gorm:"autoCreateTime"`

	Food Food `json:"food" gorm:"foreignKey:FoodID"`
}

// TableName overrides the default pluralization to map to the 000001_enterprise_schema "food_logs" table.
func (MealLog) TableName() string {
	return "food_logs"
}

// LogMealRequest is the input payload for tracking a food consumption event.
type LogMealRequest struct {
	FoodID        uuid.UUID `json:"food_id"        binding:"required"`
	QuantityGrams float64   `json:"quantity_grams" binding:"required,gt=0"`
	MealType      string    `json:"meal_type"      binding:"required"`
	ConsumedDate  string    `json:"consumed_date"  binding:"required"`
}

// CreateFoodRequest is the input payload for adding a custom food item.
type CreateFoodRequest struct {
	Name            string            `json:"name"              binding:"required"`
	Category        string            `json:"category"`
	CaloriesPer100g float64           `json:"calories_per_100g" binding:"required"`
	ProteinPer100g  float64           `json:"protein_per_100g"`
	CarbsPer100g    float64           `json:"carbs_per_100g"`
	FatPer100g      float64           `json:"fat_per_100g"`
	Micronutrients  datatypes.JSONMap `json:"micronutrients"`
	ServingSize     string            `json:"serving_size"`
	IsVegan         bool              `json:"is_vegan"`
	ImageURL        string            `json:"image_url"`
}

// DailyPlanResponse aggregates today's nutrition metrics with food suggestions.
type DailyPlanResponse struct {
	TargetCalories   float64   `json:"target_calories"`
	ConsumedCalories float64   `json:"consumed_calories"`
	BurnedCalories   float64   `json:"burned_calories"` // Total burned via workout logs today
	TargetWater      int       `json:"target_water"`    // <--- Hydration
	ConsumedWater    int       `json:"consumed_water"`  // <--- Hydration
	LoggedMeals      []MealLog `json:"logged_meals"`
	RecommendedFoods []Food    `json:"recommended_foods"`
}

// DailyAnalytics holds aggregated nutrition data for a single calendar day.
type DailyAnalytics struct {
	Date     string  `json:"date"`     // YYYY-MM-DD
	Consumed float64 `json:"consumed"` // total kcal from meal_logs
	Burned   float64 `json:"burned"`   // total kcal from workout_logs
}

// WeeklyAnalyticsResponse is the payload for GET /nutrition/analytics/weekly.
type WeeklyAnalyticsResponse struct {
	Days []DailyAnalytics `json:"days"`
}

// NutritionRepository defines the data access boundary.
type NutritionRepository interface {
	GetFoodByID(ctx context.Context, id uuid.UUID) (*Food, error)
	// Transaction Handler for UseCases
	WithTransaction(ctx context.Context, fn func(repo NutritionRepository) error) error
	SearchFoods(ctx context.Context, keyword string) ([]Food, error)
	SearchFoodsByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]Food, error)
	GetRandomFoods(ctx context.Context, limit int) ([]Food, error)
	CreateFood(ctx context.Context, food *Food) error
	UpsertFoods(ctx context.Context, foods []Food) error
	LogMeal(ctx context.Context, log *MealLog) error
	GetDailyLogs(ctx context.Context, userID uuid.UUID, date time.Time) ([]MealLog, error)
	// GetWeeklyConsumed returns per-day SUM(calories_consumed) from food_logs for the last n days.
	GetWeeklyConsumed(ctx context.Context, userID uuid.UUID, days int) (map[string]float64, error)
	// GetWeeklyBurned returns per-day SUM(calories_burned) from workout_logs for the last n days.
	GetWeeklyBurned(ctx context.Context, userID uuid.UUID, days int) (map[string]float64, error)
	GetMealLogForUpdate(ctx context.Context, logID, userID uuid.UUID) (*MealLog, error)
	UpdateMealLog(ctx context.Context, log *MealLog) error
	
	// Hydration Methods
	LogWater(ctx context.Context, log *WaterLog) error
	GetDailyConsumedWater(ctx context.Context, userID uuid.UUID, date time.Time) (int, error)
}

// NutritionUseCase defines the core business logic boundary for nutrition operations.
type NutritionUseCase interface {
	SearchFoods(ctx context.Context, keyword string) ([]Food, error)
	SearchSpoonacular(ctx context.Context, query, diet, intolerances string, maxCarbs int) ([]Food, error)
	SearchByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]Food, error)
	SearchByIngredients(ctx context.Context, ingredients string) ([]Food, error)
	CreateFood(ctx context.Context, req *CreateFoodRequest) (*Food, error)
	LogMeal(ctx context.Context, userID uuid.UUID, req *LogMealRequest) (*MealLog, error)
	GetDailyPlan(ctx context.Context, userID uuid.UUID, dateStr string) (*DailyPlanResponse, error)
	GetWeeklyAnalytics(ctx context.Context, userID uuid.UUID) (*WeeklyAnalyticsResponse, error)
	UpdateFoodLog(ctx context.Context, userID, logID uuid.UUID, quantity float64) (*MealLog, error)
	GetJobStatus(ctx context.Context, jobID string) (*JobStatusResponse, error)
	
	// Hydration Methods
	LogWater(ctx context.Context, userID uuid.UUID, req *LogWaterRequest) (*LogWaterResponse, error)
}

// JobStatusResponse represents the unified DTO for external API consumers checking orchestrator jobs.
type JobStatusResponse struct {
	ID        string
	Type      string
	Status    string
	Done      bool
	Error     string
	UpdatedAt time.Time
}
