package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/service"
)

// --- Mock Implementations for Streak Tests ---

type mockStreakRepo struct {
	streak *domain.UserStreak
}

func (m *mockStreakRepo) GetStreak(ctx context.Context, userID uuid.UUID) (*domain.UserStreak, error) {
	return m.streak, nil
}

func (m *mockStreakRepo) UpsertStreak(ctx context.Context, streak *domain.UserStreak) error {
	m.streak = streak
	return nil
}

type mockUserRepoForStreak struct {
	timezone string
}

func (m *mockUserRepoForStreak) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return &domain.User{
		ID:       id,
		WeightKg: 70,
		HeightCm: 175,
		DOB:      func() *time.Time { t := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC); return &t }(),
		Gender:   "male",
		TDEE:     2000,
		BMR:      1600,
		GoalType: domain.GoalMaintain,
		Timezone: m.timezone,
	}, nil
}

func (m *mockUserRepoForStreak) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepoForStreak) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForStreak) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepoForStreak) UpdateProfile(ctx context.Context, id uuid.UUID, req *domain.UpdateProfileRequest) error {
	return nil
}
func (m *mockUserRepoForStreak) SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error {
	return nil
}
func (m *mockUserRepoForStreak) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	return nil, nil
}
func (m *mockUserRepoForStreak) RevokeRefreshToken(ctx context.Context, oldHash string, replacedByHash *string) error {
	return nil
}
func (m *mockUserRepoForStreak) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	return nil
}

type mockNutritionRepoForStreak struct {
	firstSnapDate time.Time
	snapshots     map[time.Time]domain.DailyHealthSnapshot
	consumed      map[time.Time]int
	water         map[time.Time]int
	burned        map[time.Time]int
}

func (m *mockNutritionRepoForStreak) GetFirstSnapshotDate(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	return m.firstSnapDate, nil
}

func (m *mockNutritionRepoForStreak) GetOrCreateSnapshot(ctx context.Context, snapshot *domain.DailyHealthSnapshot) (*domain.DailyHealthSnapshot, error) {
	k := snapshot.SnapshotDate.UTC()
	if snap, exists := m.snapshots[k]; exists {
		return &snap, nil
	}
	m.snapshots[k] = *snapshot
	return snapshot, nil
}

func (m *mockNutritionRepoForStreak) GetSnapshotRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyHealthSnapshot, error) {
	var out []domain.DailyHealthSnapshot
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		k := d.UTC()
		if snap, ok := m.snapshots[k]; ok {
			out = append(out, snap)
		}
	}
	return out, nil
}

func (m *mockNutritionRepoForStreak) GetConsumedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	var out []domain.DailyCalorieAggregate
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		k := d.UTC()
		if total, ok := m.consumed[k]; ok {
			out = append(out, domain.DailyCalorieAggregate{Day: d, Total: total})
		}
	}
	return out, nil
}

func (m *mockNutritionRepoForStreak) GetBurnedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	var out []domain.DailyCalorieAggregate
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		k := d.UTC()
		if total, ok := m.burned[k]; ok {
			out = append(out, domain.DailyCalorieAggregate{Day: d, Total: total})
		}
	}
	return out, nil
}

func (m *mockNutritionRepoForStreak) GetWaterRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyWaterAggregate, error) {
	var out []domain.DailyWaterAggregate
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		k := d.UTC()
		if total, ok := m.water[k]; ok {
			out = append(out, domain.DailyWaterAggregate{Day: d, Total: total})
		}
	}
	return out, nil
}

func (m *mockNutritionRepoForStreak) GetFoodByID(ctx context.Context, id uuid.UUID) (*domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepoForStreak) WithTransaction(ctx context.Context, fn func(repo domain.NutritionRepository) error) error {
	return fn(m)
}
func (m *mockNutritionRepoForStreak) SearchFoods(ctx context.Context, keyword string) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepoForStreak) SearchFoodsByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepoForStreak) GetRandomFoods(ctx context.Context, limit int) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepoForStreak) CreateFood(ctx context.Context, food *domain.Food) error {
	return nil
}
func (m *mockNutritionRepoForStreak) UpsertFoods(ctx context.Context, foods []domain.Food) error {
	return nil
}
func (m *mockNutritionRepoForStreak) LogMeal(ctx context.Context, log *domain.MealLog) error {
	return nil
}
func (m *mockNutritionRepoForStreak) GetDailyLogs(ctx context.Context, userID uuid.UUID, date time.Time) ([]domain.MealLog, error) {
	return nil, nil
}
func (m *mockNutritionRepoForStreak) GetWeeklyConsumed(ctx context.Context, userID uuid.UUID, days int) (map[string]float64, error) {
	return nil, nil
}
func (m *mockNutritionRepoForStreak) GetWeeklyBurned(ctx context.Context, userID uuid.UUID, days int) (map[string]float64, error) {
	return nil, nil
}
func (m *mockNutritionRepoForStreak) GetMealLogForUpdate(ctx context.Context, logID, userID uuid.UUID) (*domain.MealLog, error) {
	return nil, nil
}
func (m *mockNutritionRepoForStreak) UpdateMealLog(ctx context.Context, log *domain.MealLog) error {
	return nil
}
func (m *mockNutritionRepoForStreak) LogWater(ctx context.Context, log *domain.WaterLog) error {
	return nil
}
func (m *mockNutritionRepoForStreak) GetDailyConsumedWater(ctx context.Context, userID uuid.UUID, date time.Time) (int, error) {
	return 0, nil
}

// --- Tests ---

func TestStreakEvaluation_UnboundedScanAndSelfHealing(t *testing.T) {
	userID := uuid.New()
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh") // UTC+7
	nowLocal := time.Date(2026, 6, 2, 10, 0, 0, 0, loc)

	// Create historical data
	// Let's seed 10 days of perfect history starting from 2026-05-23 local up to 2026-06-01 local.
	firstDateLocal := time.Date(2026, 5, 23, 0, 0, 0, 0, loc)
	
	snapshots := make(map[time.Time]domain.DailyHealthSnapshot)
	consumed := make(map[time.Time]int)
	water := make(map[time.Time]int)
	burned := make(map[time.Time]int)

	for d := firstDateLocal; !d.After(nowLocal); d = d.AddDate(0, 0, 1) {
		k := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
		snapshots[k] = domain.DailyHealthSnapshot{
			UserID:         userID,
			SnapshotDate:   k,
			TargetCalories: 2000,
			TargetWater:    2000,
		}
		// Seeding perfect logs to hit targets (100% Calorie & Water goals)
		consumed[k] = 2000
		water[k] = 2000
	}

	nutriRepo := &mockNutritionRepoForStreak{
		firstSnapDate: time.Date(firstDateLocal.Year(), firstDateLocal.Month(), firstDateLocal.Day(), 0, 0, 0, 0, time.UTC),
		snapshots:     snapshots,
		consumed:      consumed,
		water:         water,
		burned:        burned,
	}

	streakRepo := &mockStreakRepo{}
	userRepo := &mockUserRepoForStreak{timezone: "Asia/Ho_Chi_Minh"}
	analyticsSvc := service.NewAnalyticsAggregationService(nutriRepo, userRepo)
	streakSvc := service.NewStreakEvaluationService(nutriRepo, streakRepo, userRepo, analyticsSvc)

	// 1. Evaluate streak. Since today (2026-06-02) is complete, we expect:
	// Streak of 11 days (2026-05-23 to 2026-06-02 = 11 days).
	ctx := context.Background()
	res, err := streakSvc.EvaluateStreak(ctx, userID, nowLocal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.CurrentStreak != 11 {
		t.Errorf("expected current streak of 11, got %d", res.CurrentStreak)
	}
	if res.LongestStreak != 11 {
		t.Errorf("expected longest streak of 11, got %d", res.LongestStreak)
	}

	// 2. Verify self-healing dynamic LongestStreak
	// Break the streak in the middle (5 days ago: 2026-05-28).
	brokenDayLocal := time.Date(2026, 5, 28, 0, 0, 0, 0, loc)
	brokenDayUTC := time.Date(brokenDayLocal.Year(), brokenDayLocal.Month(), brokenDayLocal.Day(), 0, 0, 0, 0, time.UTC)
	
	// Set calorie intake to 0 on that day (fails the 0.7 * target limit)
	nutriRepo.consumed[brokenDayUTC] = 0

	// Re-evaluate. The consecutive runs are now:
	// - Run 1: 2026-05-23 to 2026-05-27 = 5 days
	// - Broken: 2026-05-28
	// - Run 2: 2026-05-29 to 2026-06-02 = 5 days
	// Current streak ending at today (2026-06-02) should be 5 days.
	// Longest streak should dynamically heal and drop from 11 to 5 days.
	res, err = streakSvc.EvaluateStreak(ctx, userID, nowLocal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.CurrentStreak != 5 {
		t.Errorf("expected current streak to heal to 5, got %d", res.CurrentStreak)
	}
	if res.LongestStreak != 5 {
		t.Errorf("expected longest streak to heal to 5, got %d", res.LongestStreak)
	}
}

func TestStreakEvaluation_TimezoneSafety(t *testing.T) {
	userID := uuid.New()
	
	// Let's verify a log event at 00:15 local time under Asia/Ho_Chi_Minh (UTC+7).
	// 00:15 local on 2026-06-02 corresponds to 17:15 UTC on 2026-06-01.
	// The evaluation must map to local day 2026-06-02.
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	nowLocal := time.Date(2026, 6, 2, 0, 15, 0, 0, loc)

	snapshots := make(map[time.Time]domain.DailyHealthSnapshot)
	consumed := make(map[time.Time]int)
	water := make(map[time.Time]int)
	burned := make(map[time.Time]int)

	// Seed only local day 2026-06-02 (UTC midnight date)
	targetDayUTC := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	snapshots[targetDayUTC] = domain.DailyHealthSnapshot{
		UserID:         userID,
		SnapshotDate:   targetDayUTC,
		TargetCalories: 2000,
		TargetWater:    2000,
	}
	consumed[targetDayUTC] = 2000
	water[targetDayUTC] = 2000

	nutriRepo := &mockNutritionRepoForStreak{
		firstSnapDate: targetDayUTC,
		snapshots:     snapshots,
		consumed:      consumed,
		water:         water,
		burned:        burned,
	}

	streakRepo := &mockStreakRepo{}
	userRepo := &mockUserRepoForStreak{timezone: "Asia/Ho_Chi_Minh"}
	analyticsSvc := service.NewAnalyticsAggregationService(nutriRepo, userRepo)
	streakSvc := service.NewStreakEvaluationService(nutriRepo, streakRepo, userRepo, analyticsSvc)

	ctx := context.Background()
	res, err := streakSvc.EvaluateStreak(ctx, userID, nowLocal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Since we seeded only 2026-06-02 and it's evaluated successfully,
	// current streak should be 1.
	if res.CurrentStreak != 1 {
		t.Errorf("expected timezone-safe current streak of 1, got %d", res.CurrentStreak)
	}
}
