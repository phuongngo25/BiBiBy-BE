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

	// KG Metadata (Sprint 1A: Search Exposure)
	KGMetadata *KGMetadata `json:"kg_metadata" gorm:"-"`
}

// KGMetadata represents Knowledge Graph safety signals for a food item.
type KGMetadata struct {
	IsSafe              bool        `json:"is_safe"`
	RiskLevel           string      `json:"risk_level"`
	Warnings            []KGWarning `json:"warnings"`
	Explanation         string      `json:"explanation"`
	RecommendationTrace []string    `json:"recommendation_trace"`
	KGVersion           string      `json:"kg_version"`
	GeneratedAt         int64       `json:"generated_at"`
}

// KGWarning represents a specific safety violation.
type KGWarning struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
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
	QuantityGrams float64   `json:"quantity_grams" binding:"required,gt=0,lte=5000"`
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

type GetRecommendationsRequest struct {
	Gaps []NutritionGap `json:"gaps"`
}

type NutritionGap struct {
	NutrientCode string  `json:"nutrient_code"`
	GapAmount    float64 `json:"gap_amount"`
	Unit         string  `json:"unit"`
}

type GetRecommendationsResponse struct {
	Recommendations []Recommendation `json:"recommendations"`
}

type Recommendation struct {
	FoodID     string                 `json:"food_id"`
	FoodName   string                 `json:"food_name"`
	MatchScore float64                `json:"match_score"`
	Reasons    []RecommendationReason `json:"reasons"`
}

type RecommendationReason struct {
	NutrientCode      string  `json:"nutrient_code"`
	ContributionScore float64 `json:"contribution_score"`
	ReasonType        string  `json:"reason_type"`
}

// DailyCalorieAggregate holds total calories consumed or burned grouped by day.
type DailyCalorieAggregate struct {
	Day   time.Time
	Total int
}

// DailyWaterAggregate holds total water consumed grouped by day.
type DailyWaterAggregate struct {
	Day   time.Time
	Total int
}

// DayAnalytics represents the unified daily health status.
type DayAnalytics struct {
	Date               string `json:"date"`                 // YYYY-MM-DD
	ConsumedCalories   int    `json:"consumed_calories"`    // Rounded integer
	TargetCalories     int    `json:"target_calories"`      // Frozen target
	ConsumedWater      int    `json:"consumed_water"`       // ml
	TargetWater        int    `json:"target_water"`         // ml
	EstimatedDailyBurn int    `json:"estimated_daily_burn"` // Frozen base TDEE
	WorkoutBurned      int    `json:"workout_burned"`       // Workout calories burned
	TotalBurned        int    `json:"total_burned"`         // EstimatedDailyBurn + WorkoutBurned
	CalorieGoalHit     bool   `json:"calorie_goal_hit"`
	WaterGoalHit       bool   `json:"water_goal_hit"`
	GoalHit            bool   `json:"goal_hit"` // CalorieGoalHit && WaterGoalHit
}

// WeeklyAnalyticsResponse is the payload for GET /nutrition/analytics/weekly.
type WeeklyAnalyticsResponse struct {
	Type                   string         `json:"type"` // "rolling" or "calendar"
	WindowDays             int            `json:"window_days"`
	StartDate              string         `json:"start_date"` // YYYY-MM-DD
	EndDate                string         `json:"end_date"`   // YYYY-MM-DD
	Days                   []DayAnalytics `json:"days"`
	WeeklyConsumedCalories int            `json:"weekly_consumed_calories"`
	WeeklyBurnedCalories   int            `json:"weekly_burned_calories"`
	WeeklyConsumedWater    int            `json:"weekly_consumed_water"`
	StreakDays             int            `json:"streak_days"`
}

// MonthlyAnalyticsResponse is the payload for GET /nutrition/analytics/monthly.
type MonthlyAnalyticsResponse struct {
	Days                  []DayAnalytics `json:"days"`
	TotalConsumedCalories int            `json:"total_consumed_calories"`
	TotalConsumedWater    int            `json:"total_consumed_water"`
	GoalHitDays           int            `json:"goal_hit_days"`
	WaterGoalHitDays      int            `json:"water_goal_hit_days"`
	CalorieGoalHitDays    int            `json:"calorie_goal_hit_days"`
	LongestGoalHitRun     int            `json:"longest_goal_hit_run"` // Max consecutive hits within range
	CurrentGoalHitRun     int            `json:"current_goal_hit_run"` // Active consecutive streak ending at the range end date (deterministic, not relative to today)
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
	UpdateFoodServingSize(ctx context.Context, foodID uuid.UUID, servingSize string) error
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

	// Snapshot Methods
	GetOrCreateSnapshot(ctx context.Context, snapshot *DailyHealthSnapshot) (*DailyHealthSnapshot, error)
	GetFirstSnapshotDate(ctx context.Context, userID uuid.UUID) (time.Time, error)

	// Range/Batch Analytics Methods (Raw data queries only)
	GetSnapshotRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]DailyHealthSnapshot, error)
	GetConsumedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]DailyCalorieAggregate, error)
	GetBurnedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]DailyCalorieAggregate, error)
	GetWaterRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]DailyWaterAggregate, error)
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
	GetDayAnalytics(ctx context.Context, userID uuid.UUID, dateStr string) (*DayAnalytics, error)
	GetWeeklyAnalytics(ctx context.Context, userID uuid.UUID, days int, isCalendar bool) (*WeeklyAnalyticsResponse, error)
	GetMonthlyAnalytics(ctx context.Context, userID uuid.UUID, monthStr string) (*MonthlyAnalyticsResponse, error)
	ExplainFood(ctx context.Context, userID uuid.UUID, foodID string) (*FoodExplanation, error)
	GetRecommendations(ctx context.Context, userID uuid.UUID, req *GetRecommendationsRequest) (*GetRecommendationsResponse, error)
	EstimateNutrition(ctx context.Context, imageBytes []byte) (*FoodEstimateResponse, error)
	GetStreak(ctx context.Context, userID uuid.UUID) (*UserStreak, error)
	UpdateFoodLog(ctx context.Context, userID, logID uuid.UUID, quantity float64) (*MealLog, error)
	GetJobStatus(ctx context.Context, jobID string) (*JobStatusResponse, error)

	// Hydration Methods
	LogWater(ctx context.Context, userID uuid.UUID, req *LogWaterRequest) (*LogWaterResponse, error)

	// Thresholds & Feedback (Epic 1)
	GetThresholdSnapshot(ctx context.Context, userID uuid.UUID) (*ThresholdSnapshot, error)
	SubmitFoodFeedback(ctx context.Context, userID uuid.UUID, req *FoodFeedbackRequest) error

	// Planner / Meal Validation (Epic 1 Chunk C)
	AnalyzeMeal(ctx context.Context, userID uuid.UUID, req *AnalyzeMealRequest) (*AnalyzeMealResponse, error)
	GenerateWeeklyPlan(ctx context.Context, userID uuid.UUID, req *GenerateWeeklyPlanRequest) (*GenerateWeeklyPlanResponse, error)
	ReoptimizePlan(ctx context.Context, userID uuid.UUID, req *ReoptimizeWeeklyPlanRequest) (*GenerateWeeklyPlanResponse, error)
}

// ─── Planner DTOs ───────────────────────────────────────────

type GenerateWeeklyPlanRequest struct {
	StartDate      string   `json:"startDate"`
	Goal           string   `json:"goal"`
	GoalType       string   `json:"goalType"`
	Strategy       string   `json:"strategy"`
	CandidateFoods []string `json:"candidate_foods"`
}

type PlannedMealDTO struct {
	MealID   string   `json:"meal_id"`
	Date     string   `json:"date"`
	FoodIDs  []string `json:"food_ids"`
	MealType string   `json:"meal_type"`
	Status   string   `json:"status"`
	FoodName string   `json:"food_name,omitempty"`
	Calories float64  `json:"calories,omitempty"`
	Protein  float64  `json:"protein,omitempty"`
	Carbs    float64  `json:"carbs,omitempty"`
	Fat      float64  `json:"fat,omitempty"`
}

type WeeklyPlanResponseDTO struct {
	PlanID string           `json:"plan_id"`
	Meals  []PlannedMealDTO `json:"meals"`
}

type GenerateWeeklyPlanResponse struct {
	WeeklyPlanResponseDTO
}

type PlannerAdjustmentDTO struct {
	Type            string `json:"type"`
	TargetDate      string `json:"targetDate,omitempty"`
	TargetMealType  string `json:"targetMealType,omitempty"`
	PreferredFoodID string `json:"preferredFoodId,omitempty"`
}

type ReoptimizeWeeklyPlanRequest struct {
	PlanID     string               `json:"planId"`
	Adjustment PlannerAdjustmentDTO `json:"adjustment"`

	// Optional: Send the current plan to avoid DB lookup for this prototype
	CurrentPlan *WeeklyPlanResponseDTO `json:"currentPlan,omitempty"`
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

// FoodEstimateResponse is the DTO for AI food estimation preview.
type FoodEstimateResponse struct {
	FoodID          uuid.UUID `json:"food_id"`
	FoodLabel       string    `json:"food_label"`
	Name            string    `json:"name"`
	Calories        float64   `json:"calories"`
	Protein         float64   `json:"protein"`
	Fat             float64   `json:"fat"`
	Carbs           float64   `json:"carbs"`
	CaloriesPer100g float64   `json:"calories_per_100g"`
	ProteinPer100g  float64   `json:"protein_per_100g"`
	FatPer100g      float64   `json:"fat_per_100g"`
	CarbsPer100g    float64   `json:"carbs_per_100g"`
	QuantityGrams   float64   `json:"quantity_grams"`
	Confidence      float64   `json:"confidence"`
	ServingSize     float64   `json:"serving_size"`
	ServingUnit     string    `json:"serving_unit"`
	Source          string    `json:"source"`
	EstimateMethod  string    `json:"estimate_method"`
}
