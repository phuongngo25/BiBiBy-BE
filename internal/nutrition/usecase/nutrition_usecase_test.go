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
	logs             []domain.MealLog
	weeklyConsumed   map[string]float64
	weeklyBurned     map[string]float64
	waterAmount      int
	waterDateQueried time.Time
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
func (m *mockNutritionRepo) LogWater(_ context.Context, _ *domain.WaterLog) error {
	return nil
}
func (m *mockNutritionRepo) GetDailyConsumedWater(_ context.Context, _ uuid.UUID, date time.Time) (int, error) {
	m.waterDateQueried = date
	return m.waterAmount, nil
}
func (m *mockNutritionRepo) GetOrCreateSnapshot(ctx context.Context, snapshot *domain.DailyHealthSnapshot) (*domain.DailyHealthSnapshot, error) {
	return snapshot, nil
}
func (m *mockNutritionRepo) GetFirstSnapshotDate(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	return time.Time{}, nil
}
func (m *mockNutritionRepo) GetSnapshotRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyHealthSnapshot, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetConsumedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	var aggregates []domain.DailyCalorieAggregate
	for k, v := range m.weeklyConsumed {
		t, _ := time.Parse("2006-01-02", k)
		aggregates = append(aggregates, domain.DailyCalorieAggregate{
			Day:   t,
			Total: int(v),
		})
	}
	return aggregates, nil
}
func (m *mockNutritionRepo) GetBurnedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	var aggregates []domain.DailyCalorieAggregate
	for k, v := range m.weeklyBurned {
		t, _ := time.Parse("2006-01-02", k)
		aggregates = append(aggregates, domain.DailyCalorieAggregate{
			Day:   t,
			Total: int(v),
		})
	}
	return aggregates, nil
}
func (m *mockNutritionRepo) GetWaterRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyWaterAggregate, error) {
	return nil, nil
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

type mockUserRepo struct {
	user *domain.User
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if m.user != nil {
		return m.user, nil
	}
	return &domain.User{TDEE: 2000.0}, nil
}
func (m *mockUserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, req *domain.UpdateProfileRequest) error {
	return nil
}
func (m *mockUserRepo) SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error {
	return nil
}
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

	uc := usecase.NewNutritionUseCase(nutriRepo, nil, nil, nil, workoutRepo, &mockUserRepo{}, nil, nil, nil)

	plan, err := uc.GetDailyPlan(ctx, userID, time.Now().Format("2006-01-02"))
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

func TestGetDailyPlan_UsesUserTimezoneForWaterAggregation(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	nutriRepo := &mockNutritionRepo{waterAmount: 150}
	workoutRepo := &mockWorkoutRepo{}
	userRepo := &mockUserRepo{user: &domain.User{
		TDEE:     2000,
		Timezone: "Asia/Ho_Chi_Minh",
	}}
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, nil, nil, workoutRepo, userRepo, nil, nil, nil)

	plan, err := uc.GetDailyPlan(ctx, userID, "2026-06-09")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if plan.ConsumedWater != 150 {
		t.Fatalf("expected consumed water from repository, got %d", plan.ConsumedWater)
	}
	if nutriRepo.waterDateQueried.Location().String() != loc.String() {
		t.Fatalf("expected water query in %s, got %s", loc, nutriRepo.waterDateQueried.Location())
	}
	if nutriRepo.waterDateQueried.Format(time.RFC3339) != "2026-06-09T00:00:00+07:00" {
		t.Fatalf("unexpected water date: %s", nutriRepo.waterDateQueried.Format(time.RFC3339))
	}
}

// TestGetWeeklyAnalytics_ExactlySevenPoints verifies that the response always
// contains exactly 7 DayAnalytics entries even when the DB has no data.
func TestGetWeeklyAnalytics_ExactlySevenPoints(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	nutriRepo := &mockNutritionRepo{} // empty → all days default to 0
	workoutRepo := &mockWorkoutRepo{}
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, nil, nil, workoutRepo, &mockUserRepo{}, nil, nil, nil)

	resp, err := uc.GetWeeklyAnalytics(ctx, userID, 7, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Days) != 7 {
		t.Errorf("expected 7 days, got %d", len(resp.Days))
	}
	for _, d := range resp.Days {
		if d.ConsumedCalories != 0 || d.WorkoutBurned != 0 {
			t.Errorf("expected zeros for empty day %s, got consumed=%d burned=%d",
				d.Date, d.ConsumedCalories, d.WorkoutBurned)
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
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, nil, nil, workoutRepo, &mockUserRepo{}, nil, nil, nil)

	resp, err := uc.GetWeeklyAnalytics(ctx, userID, 7, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Days) != 7 {
		t.Fatalf("expected 7 days, got %d", len(resp.Days))
	}

	dayMap := make(map[string]domain.DayAnalytics)
	for _, d := range resp.Days {
		dayMap[d.Date] = d
	}

	if dayMap[today].ConsumedCalories != 1200 {
		t.Errorf("today consumed: want 1200, got %d", dayMap[today].ConsumedCalories)
	}
	if dayMap[today].WorkoutBurned != 300 {
		t.Errorf("today burned: want 300, got %d", dayMap[today].WorkoutBurned)
	}
	if dayMap[yesterday].ConsumedCalories != 950 {
		t.Errorf("yesterday consumed: want 950, got %d", dayMap[yesterday].ConsumedCalories)
	}
	if dayMap[yesterday].WorkoutBurned != 0 {
		t.Errorf("yesterday burned: want 0, got %d", dayMap[yesterday].WorkoutBurned)
	}
}

func TestAIFoodRegistryCoverage(t *testing.T) {
	// The VNFoodClassifier has exactly 30 classes.
	// This test prevents developer from adding a label to the AI
	// without adding it to the AIFoodRegistry Go map.
	expectedCount := 30
	if len(usecase.AIFoodRegistry) != expectedCount {
		t.Errorf("AIFoodRegistry coverage mismatch: expected %d, got %d", expectedCount, len(usecase.AIFoodRegistry))
	}
}
