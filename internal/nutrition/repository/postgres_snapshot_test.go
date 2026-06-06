package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"nutrix-backend/config"
	"nutrix-backend/internal/domain"
	pkgdb "nutrix-backend/pkg/database"
)

func TestGetOrCreateSnapshot_Concurrency(t *testing.T) {
	// 1. Load root .env file so config can read environmental variables
	_ = godotenv.Load("../../../.env")

	cfg := config.LoadConfig()
	db, err := pkgdb.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("Skipping integration test: database connection not available: %v", err)
		return
	}

	// Clean/Ensure table schema (idempotent run)
	if err := pkgdb.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewPostgresNutritionRepository(db)
	ctx := context.Background()

	// 2. Set up dummy test entities
	testUserID := uuid.New()
	testDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// Ensure user exists first because of foreign key constraint
	user := &domain.User{
		ID:            testUserID,
		Username:      "test_snapshot_user_" + uuid.New().String()[:8],
		Email:         "test_snapshot_" + uuid.New().String()[:8] + "@example.com",
		Password:      "secure_hash",
		Gender:        "male",
		ActivityLevel: domain.ActivitySedentary,
		GoalType:      domain.GoalLoseWeight,
		WeightKg:      80,
		HeightCm:      180,
		Timezone:      "Asia/Ho_Chi_Minh",
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	defer func() {
		// Clean up created entities
		db.Exec("DELETE FROM daily_health_snapshots WHERE user_id = ?", testUserID)
		db.Exec("DELETE FROM users WHERE id = ?", testUserID)
	}()

	// 3. Spawn concurrent goroutines attempting to create the same snapshot
	const concurrencyCount = 20
	var wg sync.WaitGroup
	results := make([]*domain.DailyHealthSnapshot, concurrencyCount)
	errors := make([]error, concurrencyCount)

	startSignal := make(chan struct{})

	for i := 0; i < concurrencyCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-startSignal // sync start

			draft := &domain.DailyHealthSnapshot{
				UserID:              testUserID,
				SnapshotDate:        testDate,
				WeightKg:            80,
				ActivityLevel:       domain.ActivitySedentary,
				GoalType:            domain.GoalLoseWeight,
				BMR:                 1800,
				TDEE:                2160,
				TargetCalories:      1660,
				TargetWater:         2000,
				GoalStrategyVersion: "v1",
			}

			res, err := repo.GetOrCreateSnapshot(ctx, draft)
			results[index] = res
			errors[index] = err
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)
	wg.Wait()

	// 4. Verify results
	var successCount int
	var failedCount int

	for i := 0; i < concurrencyCount; i++ {
		if errors[i] != nil {
			t.Errorf("Goroutine %d returned error: %v", i, errors[i])
			failedCount++
		} else {
			successCount++
		}
	}

	if failedCount > 0 {
		t.Fatalf("Expected 0 failures, got %d failures under concurrent load", failedCount)
	}

	// Verify they all returned the exact same snapshot record
	firstID := results[0].ID
	for i := 1; i < concurrencyCount; i++ {
		if results[i].ID != firstID {
			t.Errorf("Mismatch in snapshot ID: goroutine 0 returned %s, but goroutine %d returned %s", firstID, i, results[i].ID)
		}
	}

	// Verify only 1 record is actually in the database
	var dbCount int64
	db.Model(&domain.DailyHealthSnapshot{}).
		Where("user_id = ? AND snapshot_date = ?", testUserID, testDate).
		Count(&dbCount)

	if dbCount != 1 {
		t.Errorf("Expected exactly 1 snapshot in DB, got %d", dbCount)
	}
}

func TestNutritionRepository_RangeQueries(t *testing.T) {
	_ = godotenv.Load("../../../.env")

	cfg := config.LoadConfig()
	db, err := pkgdb.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("Skipping integration test: database connection not available: %v", err)
		return
	}

	repo := NewPostgresNutritionRepository(db)
	ctx := context.Background()

	testUserID := uuid.New()
	user := &domain.User{
		ID:            testUserID,
		Username:      "test_range_user_" + uuid.New().String()[:8],
		Email:         "test_range_" + uuid.New().String()[:8] + "@example.com",
		Password:      "secure_hash",
		Gender:        "female",
		ActivityLevel: domain.ActivityActive,
		GoalType:      domain.GoalMaintain,
		WeightKg:      60,
		HeightCm:      165,
		Timezone:      "Asia/Ho_Chi_Minh",
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Insert a food item first so we can link food logs
	food := &domain.Food{
		ID:              uuid.New(),
		Code:            "TEST-RANGE-FOOD",
		Name:            "Range Test Food",
		CaloriesPer100g: 200.0,
		ProteinPer100g:  10.0,
		CarbsPer100g:    25.0,
		FatPer100g:      5.0,
		Source:          "custom",
		IsVerified:      true,
	}
	if err := db.Create(food).Error; err != nil {
		t.Fatalf("Failed to create test food: %v", err)
	}

	defer func() {
		// Clean up created entities
		db.Exec("DELETE FROM food_logs WHERE user_id = ?", testUserID)
		db.Exec("DELETE FROM water_logs WHERE user_id = ?", testUserID)
		db.Exec("DELETE FROM workout_logs WHERE user_id = ?", testUserID)
		db.Exec("DELETE FROM daily_health_snapshots WHERE user_id = ?", testUserID)
		db.Exec("DELETE FROM foods WHERE id = ?", food.ID)
		db.Exec("DELETE FROM users WHERE id = ?", testUserID)
	}()

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)

	// 1. Insert snapshots
	snapshots := []domain.DailyHealthSnapshot{
		{UserID: testUserID, SnapshotDate: start, TargetCalories: 1800, TargetWater: 2000, GoalStrategyVersion: "v1"},
		{UserID: testUserID, SnapshotDate: start.AddDate(0, 0, 1), TargetCalories: 1850, TargetWater: 2100, GoalStrategyVersion: "v1"},
	}
	for _, snap := range snapshots {
		if _, err := repo.GetOrCreateSnapshot(ctx, &snap); err != nil {
			t.Fatalf("Failed to insert mock snapshot: %v", err)
		}
	}

	// 2. Insert food logs (consumed calories)
	mealLogs := []domain.MealLog{
		{UserID: testUserID, FoodID: food.ID, QuantityGrams: 100, CaloriesConsumed: 200, ConsumedDate: start},
		{UserID: testUserID, FoodID: food.ID, QuantityGrams: 200, CaloriesConsumed: 400, ConsumedDate: start},
		{UserID: testUserID, FoodID: food.ID, QuantityGrams: 150, CaloriesConsumed: 300, ConsumedDate: start.AddDate(0, 0, 1)},
	}
	for _, log := range mealLogs {
		if err := repo.LogMeal(ctx, &log); err != nil {
			t.Fatalf("Failed to log test meal: %v", err)
		}
	}

	// 3. Insert water logs
	waterLogs := []domain.WaterLog{
		{UserID: testUserID, AmountMl: 500, Source: "Cup", CreatedAt: start},
		{UserID: testUserID, AmountMl: 1000, Source: "Bottle", CreatedAt: start},
		{UserID: testUserID, AmountMl: 1500, Source: "Bottle", CreatedAt: start.AddDate(0, 0, 2)},
	}
	for _, w := range waterLogs {
		if err := repo.LogWater(ctx, &w); err != nil {
			t.Fatalf("Failed to log test water: %v", err)
		}
	}

	// 4. Insert workout logs (burned calories)
	workoutLogs := []domain.WorkoutLog{
		{UserID: testUserID, ExerciseID: "EX1", ExerciseName: "Running", DurationMinutes: 30, CaloriesBurned: 350.5, LoggedAt: start.AddDate(0, 0, 1)},
	}
	for _, w := range workoutLogs {
		if err := db.Create(&w).Error; err != nil {
			t.Fatalf("Failed to create test workout: %v", err)
		}
	}

	// Verify range queries
	t.Run("GetSnapshotRange", func(t *testing.T) {
		snaps, err := repo.GetSnapshotRange(ctx, testUserID, start, end)
		if err != nil {
			t.Fatalf("GetSnapshotRange error: %v", err)
		}
		if len(snaps) != 2 {
			t.Errorf("expected 2 snapshots, got %d", len(snaps))
		}
	})

	t.Run("GetConsumedRange", func(t *testing.T) {
		consumed, err := repo.GetConsumedRange(ctx, testUserID, start, end)
		if err != nil {
			t.Fatalf("GetConsumedRange error: %v", err)
		}
		if len(consumed) != 2 {
			t.Fatalf("expected 2 consumed calorie aggregations, got %d", len(consumed))
		}
		cMap := make(map[string]int)
		for _, c := range consumed {
			cMap[c.Day.Format("2006-01-02")] = c.Total
		}
		if cMap["2026-06-01"] != 600 {
			t.Errorf("June 1 consumed: expected 600, got %d", cMap["2026-06-01"])
		}
		if cMap["2026-06-02"] != 300 {
			t.Errorf("June 2 consumed: expected 300, got %d", cMap["2026-06-02"])
		}
	})

	t.Run("GetBurnedRange", func(t *testing.T) {
		burned, err := repo.GetBurnedRange(ctx, testUserID, start, end)
		if err != nil {
			t.Fatalf("GetBurnedRange error: %v", err)
		}
		if len(burned) != 1 {
			t.Fatalf("expected 1 burned aggregate, got %d", len(burned))
		}
		if burned[0].Total != 351 {
			t.Errorf("expected 351 calories burned, got %d", burned[0].Total)
		}
	})

	t.Run("GetWaterRange", func(t *testing.T) {
		water, err := repo.GetWaterRange(ctx, testUserID, start, end)
		if err != nil {
			t.Fatalf("GetWaterRange error: %v", err)
		}
		if len(water) != 2 {
			t.Fatalf("expected 2 water aggregates, got %d", len(water))
		}
		wMap := make(map[string]int)
		for _, w := range water {
			wMap[w.Day.Format("2006-01-02")] = w.Total
		}
		if wMap["2026-06-01"] != 1500 {
			t.Errorf("June 1 water: expected 1500, got %d", wMap["2026-06-01"])
		}
		if wMap["2026-06-03"] != 1500 {
			t.Errorf("June 3 water: expected 1500, got %d", wMap["2026-06-03"])
		}
	})
}
