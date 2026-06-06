package repository

import (
	"context"
	"errors"
	"log"
	"math"
	"strings"
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

// SearchFoods implements Fuzzy Logic + Relevance Ranking against the local DB.
//
// Matching rules (OR — any match qualifies):
//   - name_en ILIKE '%query%'   (substring, case-insensitive)
//   - name_vi ILIKE '%query%'
//   - similarity(name_en, query) > 0.2   (pg_trgm fuzzy — catches typos)
//
// Relevance ordering (best match first):
//  1. Exact match        → name_en ILIKE 'query'     (score 0)
//  2. Prefix match       → name_en ILIKE 'query%'    (score 1)
//  3. Fuzzy/substring    → ranked by similarity() DESC
//  3b. Length tiebreaker → LENGTH(name_en) ASC
//
// Hard cap: 50 results.
func (r *postgresNutritionRepository) SearchFoods(ctx context.Context, keyword string) ([]domain.Food, error) {
	if len(strings.TrimSpace(keyword)) < 2 {
		return []domain.Food{}, nil
	}

	const searchSQL = `
		SELECT * FROM foods
		WHERE
			name_en ILIKE '%' || ? || '%'
			OR name_vi ILIKE '%' || ? || '%'
			OR similarity(name_en, ?) > 0.2
		ORDER BY
			(name_en ILIKE ?)                DESC,
			(name_en ILIKE ? || '%')         DESC,
			similarity(name_en, ?)           DESC,
			LENGTH(name_en)                  ASC
		LIMIT 50
	`

	var foods []domain.Food
	err := r.db.WithContext(ctx).Raw(
		searchSQL,
		keyword, // ILIKE '%?%' name_en
		keyword, // ILIKE '%?%' name_vi
		keyword, // similarity(name_en, ?)
		keyword, // exact: name_en ILIKE ?
		keyword, // prefix: name_en ILIKE ?%
		keyword, // similarity ORDER BY
	).Scan(&foods).Error

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

	log.Printf("[DAILY_PLAN] queryStart=%v", startOfDay)
	log.Printf("[DAILY_PLAN] queryEnd=%v", endOfDay)

	err := r.db.WithContext(ctx).
		Preload("Food").
		Where("user_id = ? AND consumed_date >= ? AND consumed_date < ?", userID, startOfDay, endOfDay).
		Find(&logs).Error

	if err != nil {
		return nil, domain.ErrInternalServerError
	}

	log.Printf("[DAILY_PLAN] mealsReturned=%d", len(logs))
	for _, meal := range logs {
		log.Printf("[DAILY_PLAN] meal=%s consumedDate=%v", meal.Food.Name, meal.ConsumedDate)
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

type dailySum struct {
	Day   string  `gorm:"column:day"`
	Total float64 `gorm:"column:total"`
}

// GetWeeklyConsumed returns SUM(calories_consumed) per calendar day from food_logs
// for the given user over the last `days` days (inclusive of today).
func (r *postgresNutritionRepository) GetWeeklyConsumed(ctx context.Context, userID uuid.UUID, days int) (map[string]float64, error) {
	var rows []dailySum
	err := r.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(consumed_date, 'YYYY-MM-DD') AS day,
		       COALESCE(SUM(calories_consumed), 0)  AS total
		FROM   food_logs
		WHERE  user_id      = ?
		  AND  consumed_date >= CURRENT_DATE - INTERVAL '1 day' * ?
		GROUP  BY day
		ORDER  BY day
	`, userID, days-1).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, r := range rows {
		out[r.Day] = r.Total
	}
	return out, nil
}

// GetWeeklyBurned returns SUM(calories_burned) per calendar day from workout_logs
// for the given user over the last `days` days (inclusive of today).
func (r *postgresNutritionRepository) GetWeeklyBurned(ctx context.Context, userID uuid.UUID, days int) (map[string]float64, error) {
	var rows []dailySum
	err := r.db.WithContext(ctx).Raw(`
		SELECT TO_CHAR(DATE(logged_at), 'YYYY-MM-DD') AS day,
		       COALESCE(SUM(calories_burned), 0)       AS total
		FROM   workout_logs
		WHERE  user_id   = ?
		  AND  DATE(logged_at) >= CURRENT_DATE - INTERVAL '1 day' * ?
		GROUP  BY day
		ORDER  BY day
	`, userID, days-1).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, r := range rows {
		out[r.Day] = r.Total
	}
	return out, nil
}

func (r *postgresNutritionRepository) GetMealLogForUpdate(ctx context.Context, logID, userID uuid.UUID) (*domain.MealLog, error) {
	var log domain.MealLog
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Food").
		First(&log, "id = ? AND user_id = ?", logID, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrLogNotFound
		}
		return nil, domain.ErrInternalServerError
	}
	return &log, nil
}

func (r *postgresNutritionRepository) UpdateMealLog(ctx context.Context, log *domain.MealLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

// LogWater inserts a WaterLog record into the database.
func (r *postgresNutritionRepository) LogWater(ctx context.Context, log *domain.WaterLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetDailyConsumedWater fetches the sum of all water amounts for a given user on a calendar day.
func (r *postgresNutritionRepository) GetDailyConsumedWater(ctx context.Context, userID uuid.UUID, date time.Time) (int, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var total int
	err := r.db.WithContext(ctx).
		Model(&domain.WaterLog{}).
		Select("COALESCE(SUM(amount_ml), 0)").
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, startOfDay, endOfDay).
		Scan(&total).Error

	if err != nil {
		return 0, domain.ErrInternalServerError
	}
	return total, nil
}

// GetOrCreateSnapshot implements the race-safe retrieval/creation of daily health snapshots.
// NOTE: This may be called from read-only query endpoints (like Analytics) to dynamically
// auto-backfill and freeze missing snapshots. This read-side mutation is a deliberate
// eventually self-healing architectural decision to guarantee historical goal stability.
// Operation is 100% idempotent.
func (r *postgresNutritionRepository) GetOrCreateSnapshot(ctx context.Context, snapshot *domain.DailyHealthSnapshot) (*domain.DailyHealthSnapshot, error) {
	// 1. Attempt UPSERT / DO NOTHING
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "snapshot_date"}},
			DoNothing: true,
		}).
		Create(snapshot).Error

	if err != nil {
		return nil, err
	}

	// 2. Always fetch the true single source of truth for that day
	var existing domain.DailyHealthSnapshot
	err = r.db.WithContext(ctx).
		Where("user_id = ? AND snapshot_date = ?", snapshot.UserID, snapshot.SnapshotDate).
		First(&existing).Error

	return &existing, err
}

// GetSnapshotRange retrieves daily health snapshots for a user over a date range.
func (r *postgresNutritionRepository) GetSnapshotRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyHealthSnapshot, error) {
	var snapshots []domain.DailyHealthSnapshot
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND snapshot_date >= ? AND snapshot_date <= ?", userID, start, end).
		Order("snapshot_date ASC").
		Find(&snapshots).Error
	return snapshots, err
}

// GetConsumedRange aggregates consumed calories from food logs over a date range.
func (r *postgresNutritionRepository) GetConsumedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	var rows []struct {
		Day   time.Time
		Total float64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT DATE(consumed_date) AS day,
		       COALESCE(SUM(calories_consumed), 0) AS total
		FROM   food_logs
		WHERE  user_id = ? AND DATE(consumed_date) >= DATE(?) AND DATE(consumed_date) <= DATE(?)
		GROUP  BY day
		ORDER  BY day
	`, userID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	
	aggregates := make([]domain.DailyCalorieAggregate, len(rows))
	for i, r := range rows {
		aggregates[i] = domain.DailyCalorieAggregate{
			Day:   r.Day,
			Total: int(math.Round(r.Total)),
		}
	}
	return aggregates, nil
}

// GetBurnedRange aggregates burned calories from workout logs over a date range.
func (r *postgresNutritionRepository) GetBurnedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	var rows []struct {
		Day   time.Time
		Total float64
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT DATE(logged_at) AS day,
		       COALESCE(SUM(calories_burned), 0) AS total
		FROM   workout_logs
		WHERE  user_id = ? AND DATE(logged_at) >= DATE(?) AND DATE(logged_at) <= DATE(?)
		GROUP  BY day
		ORDER  BY day
	`, userID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	
	aggregates := make([]domain.DailyCalorieAggregate, len(rows))
	for i, r := range rows {
		aggregates[i] = domain.DailyCalorieAggregate{
			Day:   r.Day,
			Total: int(math.Round(r.Total)),
		}
	}
	return aggregates, nil
}

// GetWaterRange aggregates consumed water from water logs over a date range.
func (r *postgresNutritionRepository) GetWaterRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyWaterAggregate, error) {
	var rows []struct {
		Day   time.Time
		Total int
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT DATE(created_at) AS day,
		       COALESCE(SUM(amount_ml), 0) AS total
		FROM   water_logs
		WHERE  user_id = ? AND DATE(created_at) >= DATE(?) AND DATE(created_at) <= DATE(?)
		GROUP  BY day
		ORDER  BY day
	`, userID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	
	aggregates := make([]domain.DailyWaterAggregate, len(rows))
	for i, r := range rows {
		aggregates[i] = domain.DailyWaterAggregate{
			Day:   r.Day,
			Total: r.Total,
		}
	}
	return aggregates, nil
}

// GetFirstSnapshotDate returns the SnapshotDate of the earliest snapshot for the user.
// Returns a zero-value time.Time if no snapshots exist.
func (r *postgresNutritionRepository) GetFirstSnapshotDate(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	var dates []time.Time
	err := r.db.WithContext(ctx).
		Model(&domain.DailyHealthSnapshot{}).
		Where("user_id = ?", userID).
		Order("snapshot_date ASC").
		Limit(1).
		Pluck("snapshot_date", &dates).Error

	if err != nil {
		return time.Time{}, err
	}
	if len(dates) == 0 {
		return time.Time{}, nil
	}
	return dates[0], nil
}


