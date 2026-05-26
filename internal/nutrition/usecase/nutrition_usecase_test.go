package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/usecase"
)

// --- Mock Repositories ---

type mockNutritionRepo struct {
	logs           []domain.MealLog
	weeklyConsumed map[string]float64
	weeklyBurned   map[string]float64
}

func (m *mockNutritionRepo) GetMealLogForUpdate(ctx context.Context, userID uuid.UUID, logID uuid.UUID) (*domain.MealLog, error) {
	return nil, nil
}

func (m *mockNutritionRepo) UpdateMealLog(ctx context.Context, log *domain.MealLog) error {
	return nil
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
	return []domain.Food{}, nil
}
func (m *mockNutritionRepo) CreateFood(ctx context.Context, food *domain.Food) error { return nil }
func (m *mockNutritionRepo) UpsertFoods(ctx context.Context, foods []domain.Food) error {
	return nil
}
func (m *mockNutritionRepo) LogMeal(ctx context.Context, log *domain.MealLog) error { return nil }
func (m *mockNutritionRepo) GetDailyLogs(ctx context.Context, userID uuid.UUID, date time.Time) ([]domain.MealLog, error) {
	return m.logs, nil
}
func (m *mockNutritionRepo) GetWeeklyConsumed(_ context.Context, _ uuid.UUID, _ int) (map[string]float64, error) {
	return m.weeklyConsumed, nil
}
func (m *mockNutritionRepo) GetWeeklyBurned(_ context.Context, _ uuid.UUID, _ int) (map[string]float64, error) {
	return m.weeklyBurned, nil
}

type mockWorkoutRepo struct {
	burned float64
}

func (m *mockWorkoutRepo) GetExercisesByMuscle(ctx context.Context, muscle string) ([]domain.Exercise, error) {
	return nil, nil
}
func (m *mockWorkoutRepo) UpsertExercises(ctx context.Context, exercises []domain.Exercise) error {
	return nil
}
func (m *mockWorkoutRepo) CountByMuscle(ctx context.Context, muscle string) (int64, error) {
	return 0, nil
}
func (m *mockWorkoutRepo) LogWorkout(ctx context.Context, log *domain.WorkoutLog) error { return nil }
func (m *mockWorkoutRepo) GetDailyBurnedCalories(ctx context.Context, userID uuid.UUID, date time.Time) (float64, error) {
	return m.burned, nil
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
	return &domain.User{TDEE: 2000.0}, nil
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

func TestGetDailyPlan_BurnedCalories(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	nutriRepo := &mockNutritionRepo{
		logs: []domain.MealLog{
			{CaloriesConsumed: 500},
			{CaloriesConsumed: 300},
		},
	}
	workoutRepo := &mockWorkoutRepo{burned: 450.5}

	uc := usecase.NewNutritionUseCase(nutriRepo, nil, workoutRepo, &mockUserRepo{})

	plan, err := uc.GetDailyPlan(ctx, userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if plan.ConsumedCalories != 800 {
		t.Errorf("expected 800 consumed calories, got %f", plan.ConsumedCalories)
	}
	if plan.BurnedCalories != 450.5 {
		t.Errorf("expected 450.5 burned calories, got %f", plan.BurnedCalories)
	}
}

// TestGetWeeklyAnalytics_ExactlySevenPoints verifies that the response always
// contains exactly 7 DailyAnalytics entries even when the DB has no data.
func TestGetWeeklyAnalytics_ExactlySevenPoints(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	nutriRepo := &mockNutritionRepo{} // empty → all days default to 0
	workoutRepo := &mockWorkoutRepo{}
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, workoutRepo, &mockUserRepo{})

	resp, err := uc.GetWeeklyAnalytics(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Days) != 7 {
		t.Errorf("expected 7 days, got %d", len(resp.Days))
	}
	for _, d := range resp.Days {
		if d.Consumed != 0 || d.Burned != 0 {
			t.Errorf("expected zeros for empty day %s, got consumed=%f burned=%f",
				d.Date, d.Consumed, d.Burned)
		}
	}
}

// TestGetWeeklyAnalytics_SumsCorrect verifies correct value mapping and gap-fill.
func TestGetWeeklyAnalytics_SumsCorrect(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	nutriRepo := &mockNutritionRepo{
		weeklyConsumed: map[string]float64{
			today:     1200.0,
			yesterday: 950.5,
		},
		weeklyBurned: map[string]float64{
			today: 300.0,
		},
	}
	workoutRepo := &mockWorkoutRepo{}
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, workoutRepo, &mockUserRepo{})

	resp, err := uc.GetWeeklyAnalytics(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Days) != 7 {
		t.Fatalf("expected 7 days, got %d", len(resp.Days))
	}

	dayMap := make(map[string]domain.DailyAnalytics)
	for _, d := range resp.Days {
		dayMap[d.Date] = d
	}

	if dayMap[today].Consumed != 1200.0 {
		t.Errorf("today consumed: want 1200.0, got %f", dayMap[today].Consumed)
	}
	if dayMap[today].Burned != 300.0 {
		t.Errorf("today burned: want 300.0, got %f", dayMap[today].Burned)
	}
	if dayMap[yesterday].Consumed != 950.5 {
		t.Errorf("yesterday consumed: want 950.5, got %f", dayMap[yesterday].Consumed)
	}
	if dayMap[yesterday].Burned != 0 {
		t.Errorf("yesterday burned: want 0, got %f", dayMap[yesterday].Burned)
	}
}
