package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
)

// --- Mock Repositories ---

type mockNutritionRepo struct {
	snapshots []domain.DailyHealthSnapshot
	consumed  []domain.DailyCalorieAggregate
	burned    []domain.DailyCalorieAggregate
	water     []domain.DailyWaterAggregate
}

func (m *mockNutritionRepo) GetFoodByID(ctx context.Context, id uuid.UUID) (*domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepo) WithTransaction(ctx context.Context, fn func(repo domain.NutritionRepository) error) error {
	return fn(m)
}
func (m *mockNutritionRepo) SearchFoods(ctx context.Context, keyword string) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepo) SearchFoodsByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetRandomFoods(ctx context.Context, limit int) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepo) CreateFood(ctx context.Context, food *domain.Food) error { return nil }
func (m *mockNutritionRepo) UpsertFoods(ctx context.Context, foods []domain.Food) error { return nil }
func (m *mockNutritionRepo) LogMeal(ctx context.Context, log *domain.MealLog) error   { return nil }
func (m *mockNutritionRepo) GetDailyLogs(ctx context.Context, userID uuid.UUID, date time.Time) ([]domain.MealLog, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetWeeklyConsumed(ctx context.Context, userID uuid.UUID, days int) (map[string]float64, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetWeeklyBurned(ctx context.Context, userID uuid.UUID, days int) (map[string]float64, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetMealLogForUpdate(ctx context.Context, logID, userID uuid.UUID) (*domain.MealLog, error) {
	return nil, nil
}
func (m *mockNutritionRepo) UpdateMealLog(ctx context.Context, log *domain.MealLog) error {
	return nil
}
func (m *mockNutritionRepo) LogWater(ctx context.Context, log *domain.WaterLog) error { return nil }
func (m *mockNutritionRepo) GetDailyConsumedWater(ctx context.Context, userID uuid.UUID, date time.Time) (int, error) {
	return 0, nil
}
func (m *mockNutritionRepo) GetOrCreateSnapshot(ctx context.Context, snapshot *domain.DailyHealthSnapshot) (*domain.DailyHealthSnapshot, error) {
	return snapshot, nil
}
func (m *mockNutritionRepo) GetFirstSnapshotDate(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	return time.Time{}, nil
}

func (m *mockNutritionRepo) GetSnapshotRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyHealthSnapshot, error) {
	return m.snapshots, nil
}
func (m *mockNutritionRepo) GetConsumedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	return m.consumed, nil
}
func (m *mockNutritionRepo) GetBurnedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	return m.burned, nil
}
func (m *mockNutritionRepo) GetWaterRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyWaterAggregate, error) {
	return m.water, nil
}

type mockUserRepo struct{}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return &domain.User{
		ID:            id,
		BMR:           1500,
		TDEE:          2000,
		ActivityLevel: domain.ActivitySedentary,
		GoalType:      domain.GoalLoseWeight,
		WeightKg:      70,
	}, nil
}
func (m *mockUserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, req *domain.UpdateProfileRequest) error {
	return nil
}
func (m *mockUserRepo) SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error { return nil }
func (m *mockUserRepo) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	return nil, nil
}
func (m *mockUserRepo) RevokeRefreshToken(ctx context.Context, oldHash string, replacedByHash *string) error {
	return nil
}
func (m *mockUserRepo) RevokeFamily(ctx context.Context, familyID uuid.UUID) error { return nil }

// --- Tests ---

func TestAnalyticsAggregationService_BuildAnalyticsRange(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC) // 3-day range

	nutriRepo := &mockNutritionRepo{
		snapshots: []domain.DailyHealthSnapshot{
			{
				SnapshotDate:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				TargetCalories: 2000,
				TargetWater:    2500,
				TDEE:           2000,
			},
			{
				SnapshotDate:   time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
				TargetCalories: 1800,
				TargetWater:    2200,
				TDEE:           1800,
			},
		},
		consumed: []domain.DailyCalorieAggregate{
			{
				Day:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				Total: 1500, // 0.7 * 2000 = 1400 (hit!)
			},
			{
				Day:   time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
				Total: 1000, // 0.7 * 1800 = 1260 (not hit!)
			},
		},
		burned: []domain.DailyCalorieAggregate{
			{
				Day:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				Total: 300,
			},
		},
		water: []domain.DailyWaterAggregate{
			{
				Day:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
				Total: 2600, // >= 2500 (hit!)
			},
		},
	}

	service := NewAnalyticsAggregationService(nutriRepo, &mockUserRepo{})

	days, err := service.BuildAnalyticsRange(ctx, userID, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Verify gap-filling: range is June 1st to June 3rd (exactly 3 days)
	if len(days) != 3 {
		t.Fatalf("expected 3 days from gap-filling, got %d", len(days))
	}

	// June 1st: snap exists, consumed exists (1500), water exists (2600)
	day1 := days[0]
	if day1.Date != "2026-06-01" {
		t.Errorf("expected date 2026-06-01, got %s", day1.Date)
	}
	if day1.ConsumedCalories != 1500 || day1.TargetCalories != 2000 {
		t.Errorf("mismatch day 1 calories: consumed=%d, target=%d", day1.ConsumedCalories, day1.TargetCalories)
	}
	if day1.WorkoutBurned != 300 || day1.TotalBurned != 2300 {
		t.Errorf("mismatch day 1 burned: workout=%d, total=%d", day1.WorkoutBurned, day1.TotalBurned)
	}
	if !day1.CalorieGoalHit || !day1.WaterGoalHit || !day1.GoalHit {
		t.Errorf("expected day 1 to be fully GoalHit")
	}

	// June 2nd: snap missing (gap-filled!), consumed missing, water missing (Read-Only target should be 0)
	day2 := days[1]
	if day2.Date != "2026-06-02" {
		t.Errorf("expected date 2026-06-02, got %s", day2.Date)
	}
	if day2.ConsumedCalories != 0 || day2.TargetCalories != 0 {
		t.Errorf("mismatch day 2 calories: consumed=%d, target=%d", day2.ConsumedCalories, day2.TargetCalories)
	}
	if day2.CalorieGoalHit || day2.WaterGoalHit || day2.GoalHit {
		t.Errorf("expected day 2 to NOT hit goals")
	}

	// June 3rd: snap exists, consumed exists (1000 - no hit), water missing (no hit)
	day3 := days[2]
	if day3.Date != "2026-06-03" {
		t.Errorf("expected date 2026-06-03, got %s", day3.Date)
	}
	if day3.ConsumedCalories != 1000 || day3.TargetCalories != 1800 {
		t.Errorf("mismatch day 3 calories: consumed=%d, target=%d", day3.ConsumedCalories, day3.TargetCalories)
	}
	if day3.CalorieGoalHit || day3.WaterGoalHit || day3.GoalHit {
		t.Errorf("expected day 3 to NOT hit goals")
	}
}
