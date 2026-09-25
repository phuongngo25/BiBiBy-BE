package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
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
	foodsByID        map[uuid.UUID]domain.Food
	dailyLogsErr     error
}

func (m *mockNutritionRepo) GetMealLogForUpdate(ctx context.Context, userID uuid.UUID, logID uuid.UUID) (*domain.MealLog, error) {
	return nil, nil
}

func (m *mockNutritionRepo) UpdateMealLog(ctx context.Context, log *domain.MealLog) error {
	return nil
}

func (m *mockNutritionRepo) GetFoodByID(ctx context.Context, id uuid.UUID) (*domain.Food, error) {
	if food, ok := m.foodsByID[id]; ok {
		return &food, nil
	}
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
func (m *mockNutritionRepo) UpdateFoodServingSize(ctx context.Context, foodID uuid.UUID, servingSize string) error {
	if food, ok := m.foodsByID[foodID]; ok {
		food.ServingSize = servingSize
		m.foodsByID[foodID] = food
	}
	return nil
}
func (m *mockNutritionRepo) LogMeal(ctx context.Context, log *domain.MealLog) error { return nil }
func (m *mockNutritionRepo) GetDailyLogs(ctx context.Context, userID uuid.UUID, date time.Time) ([]domain.MealLog, error) {
	if m.dailyLogsErr != nil {
		return nil, m.dailyLogsErr
	}
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
	// getErr, when set, makes GetByID fail (an unreadable profile, plan 10-04).
	getErr error
	// nilUser makes GetByID return (nil, nil).
	nilUser bool
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.nilUser {
		return nil, nil
	}
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

func TestLogMeal_BlocksFoodConflictingWithAllergy(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	foodID := uuid.New()

	nutriRepo := &mockNutritionRepo{
		foodsByID: map[uuid.UUID]domain.Food{
			foodID: {ID: foodID, Name: "Grilled shrimp", Source: "VFA_DISH", CaloriesPer100g: 120},
		},
	}
	userRepo := &mockUserRepo{user: &domain.User{Allergies: "shellfish"}}
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, nil, nil, &mockWorkoutRepo{}, userRepo, nil, nil, nil)

	req := &domain.LogMealRequest{
		FoodID:        foodID,
		QuantityGrams: 100,
		MealType:      "lunch",
		ConsumedDate:  time.Now().Format("2006-01-02"),
	}

	// Without acknowledgement -> blocked.
	_, err := uc.LogMeal(ctx, userID, req)
	var blocked *domain.MealBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected MealBlockedError, got %v", err)
	}
	if len(blocked.Violations) == 0 {
		t.Fatalf("expected violations to be reported")
	}

	// With explicit acknowledgement -> allowed.
	req.AcknowledgedRisk = true
	if _, err := uc.LogMeal(ctx, userID, req); err != nil {
		t.Fatalf("expected acknowledged log to succeed, got %v", err)
	}
}

func TestLogMeal_AllowsSafeFood(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	foodID := uuid.New()

	nutriRepo := &mockNutritionRepo{
		foodsByID: map[uuid.UUID]domain.Food{
			foodID: {ID: foodID, Name: "Steamed rice", Source: "VFA", CaloriesPer100g: 130},
		},
	}
	userRepo := &mockUserRepo{user: &domain.User{Allergies: "shellfish"}}
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, nil, nil, &mockWorkoutRepo{}, userRepo, nil, nil, nil)

	req := &domain.LogMealRequest{
		FoodID:        foodID,
		QuantityGrams: 150,
		MealType:      "lunch",
		ConsumedDate:  time.Now().Format("2006-01-02"),
	}
	if _, err := uc.LogMeal(ctx, userID, req); err != nil {
		t.Fatalf("expected safe food to log, got %v", err)
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

func TestAnalyzeMeal_LocalEnrichmentRejectsVegetarianPhoBo(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	foodID := uuid.New()
	nutriRepo := &mockNutritionRepo{
		foodsByID: map[uuid.UUID]domain.Food{
			foodID: {
				ID:     foodID,
				Name:   "Pho Thin",
				NameVi: "Pho Thin",
				Source: "VFA_DISH",
			},
		},
	}
	userRepo := &mockUserRepo{user: &domain.User{
		TDEE:              2000,
		DietaryPreference: "vegetarian",
	}}
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, nil, nil, &mockWorkoutRepo{}, userRepo, nil, nil, nil)

	resp, err := uc.AnalyzeMeal(ctx, userID, &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{
			MealID:   "manual-lunch",
			FoodIDs:  []string{foodID.String()},
			MealType: "lunch",
		},
	})
	if err != nil {
		t.Fatalf("expected local validation without KG error, got %v", err)
	}
	if resp.Safe {
		t.Fatalf("expected unsafe vegetarian Pho Thin response")
	}
	if resp.HighestRisk != "CRITICAL" {
		t.Fatalf("expected CRITICAL, got %s", resp.HighestRisk)
	}
	if len(resp.Violations) == 0 || resp.Violations[0].ViolationType != "diet_vegetarian" {
		t.Fatalf("expected diet_vegetarian violation, got %+v", resp.Violations)
	}
	if resp.Enrichment == nil || len(resp.Enrichment.Ingredients) == 0 {
		t.Fatalf("expected ingredient enrichment, got %+v", resp.Enrichment)
	}
}

// mockKgPort embeds the interface so only AnalyzeMeal needs implementing;
// any other method call would panic (nil embedded value), which is fine —
// these tests never exercise them.
type mockKgPort struct {
	domain.NutritionIntelligencePort
	analyzeMealResp *domain.AnalyzeMealResponse
	analyzeMealErr  error
	lastReq         *domain.AnalyzeMealRequest
}

func (m *mockKgPort) AnalyzeMeal(ctx context.Context, req *domain.AnalyzeMealRequest) (*domain.AnalyzeMealResponse, error) {
	m.lastReq = req
	return m.analyzeMealResp, m.analyzeMealErr
}

// TestAnalyzeMeal_RealEnrichmentSurvivesCatalogFallback verifies the priority
// fix from Phase B4: once the AI_server gRPC call returns a real, LLM-derived
// enrichment, it must NOT be overwritten by analyzeMealFromCatalog()'s
// hardcoded switch-case — that fallback is for when gRPC has nothing to say
// about enrichment, not a competing "always wins" source.
func TestAnalyzeMeal_RealEnrichmentSurvivesCatalogFallback(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	foodID := uuid.New()
	nutriRepo := &mockNutritionRepo{
		foodsByID: map[uuid.UUID]domain.Food{
			foodID: {
				ID:     foodID,
				Name:   "Green rice sweet dessert",
				NameVi: "Che com",
				Source: "VFA_DISH",
			},
		},
	}
	userRepo := &mockUserRepo{user: &domain.User{TDEE: 2000}}
	kgPort := &mockKgPort{
		analyzeMealResp: &domain.AnalyzeMealResponse{
			Status: "APPROVED",
			Safe:   true,
			Enrichment: &domain.MealEnrichment{
				DishName:              "Che com",
				EstimatedTotalWeightG: 474,
				Source:                "llm",
				Confidence:            0.9,
				Ingredients: []domain.MealIngredientEstimate{
					{Name: "young green rice", WeightG: 200},
				},
			},
		},
	}
	uc := usecase.NewNutritionUseCase(nutriRepo, nil, nil, nil, &mockWorkoutRepo{}, userRepo, nil, kgPort, nil)

	resp, err := uc.AnalyzeMeal(ctx, userID, &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{
			MealID:   "manual-lunch",
			FoodIDs:  []string{foodID.String()},
			MealType: "lunch",
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Enrichment == nil {
		t.Fatalf("expected enrichment in response")
	}
	if resp.Enrichment.Source != "llm" {
		t.Fatalf("expected real LLM enrichment (source=llm) to survive, got source=%q dish=%q weight=%v",
			resp.Enrichment.Source, resp.Enrichment.DishName, resp.Enrichment.EstimatedTotalWeightG)
	}
	if resp.Enrichment.EstimatedTotalWeightG != 474 {
		t.Fatalf("expected the gRPC-provided weight (474g) not the catalog fallback's, got %v",
			resp.Enrichment.EstimatedTotalWeightG)
	}

	// FoodRefs must be resolved from Postgres before the gRPC call, so
	// AI_server has a real dish name to enrich with instead of a bare UUID.
	if kgPort.lastReq == nil || len(kgPort.lastReq.Candidate.FoodRefs) != 1 {
		t.Fatalf("expected FoodRefs populated before the gRPC call, got %+v", kgPort.lastReq)
	}
	if kgPort.lastReq.Candidate.FoodRefs[0].Name != "Green rice sweet dessert" {
		t.Fatalf("expected resolved food name in FoodRefs, got %+v", kgPort.lastReq.Candidate.FoodRefs[0])
	}
}

// --- Phase 10: threshold findings survive the local-catalog fallback ---

func thresholdFloat(v float64) *float64 { return &v }

func sodiumThresholdViolation(foodID, severity string) domain.MealViolation {
	return domain.MealViolation{
		ViolationType:    "nutrient_threshold",
		Description:      "Sodium: projected 1800 mg today meets or exceeds your limit of 1500 mg",
		Severity:         severity,
		OffendingFoodIDs: []string{foodID},
		NutrientID:       "sodium",
		ProjectedMg:      thresholdFloat(1800),
		ThresholdMg:      thresholdFloat(1500),
	}
}

func sodiumNutrientResult(foodID, status string) domain.NutrientThresholdResult {
	return domain.NutrientThresholdResult{
		NutrientID:          "sodium",
		Status:              status,
		ThisMealMg:          thresholdFloat(1800),
		ProjectedMg:         thresholdFloat(1800),
		Complete:            true,
		WarningMg:           1000,
		CriticalMg:          1500,
		ContributingFoodIDs: []string{foodID},
	}
}

// kgFailureViolation is worded the way AI_server's MealValidationEngine
// reports a food with no :Food node (meal_validation.py evaluate_candidate_meal:
// "Failed to evaluate food {food_id}: {error}"), which is exactly what Go's
// mealAnalysisUnavailable treats as a discard trigger.
func kgFailureViolation(foodID string) domain.MealViolation {
	return domain.MealViolation{
		ViolationType:    "safety_violation",
		Description:      fmt.Sprintf("Failed to evaluate food %s: Food %s not found in knowledge graph", foodID, foodID),
		Severity:         "RISK_LEVEL_UNSPECIFIED",
		OffendingFoodIDs: []string{foodID},
	}
}

func greenRiceRepo(foodID uuid.UUID) *mockNutritionRepo {
	return &mockNutritionRepo{foodsByID: map[uuid.UUID]domain.Food{
		foodID: {ID: foodID, Name: "Green rice sweet dessert", NameVi: "Che com", Source: "VFA_DISH"},
	}}
}

func analyzeWithKg(t *testing.T, repo *mockNutritionRepo, user *domain.User, kg *mockKgPort, foodID uuid.UUID) *domain.AnalyzeMealResponse {
	t.Helper()
	uc := usecase.NewNutritionUseCase(repo, nil, nil, nil, &mockWorkoutRepo{}, &mockUserRepo{user: user}, nil, kg, nil)
	resp, err := uc.AnalyzeMeal(context.Background(), uuid.New(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "manual-lunch", FoodIDs: []string{foodID.String()}, MealType: "lunch"},
	})
	if err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}
	return resp
}

func violationsOfType(resp *domain.AnalyzeMealResponse, violationType string) []domain.MealViolation {
	var out []domain.MealViolation
	for _, v := range resp.Violations {
		if v.ViolationType == violationType {
			out = append(out, v)
		}
	}
	return out
}

func TestAnalyzeMeal_ThresholdSurvivesLocalViolation(t *testing.T) {
	foodID := uuid.New()
	repo := &mockNutritionRepo{foodsByID: map[uuid.UUID]domain.Food{
		foodID: {ID: foodID, Name: "Pho Thin", NameVi: "Pho Thin", Source: "VFA_DISH"},
	}}
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{
		Status:          "WARNING",
		Violations:      []domain.MealViolation{sodiumThresholdViolation(foodID.String(), "RISK_LEVEL_HIGH")},
		NutrientResults: []domain.NutrientThresholdResult{sodiumNutrientResult(foodID.String(), "WARNING")},
	}}
	resp := analyzeWithKg(t, repo, &domain.User{TDEE: 2000, DietaryPreference: "vegetarian"}, kg, foodID)

	if resp.Status != "REJECTED" {
		t.Fatalf("expected local REJECTED to win, got %s", resp.Status)
	}
	if len(violationsOfType(resp, "diet_vegetarian")) == 0 {
		t.Fatalf("expected the local diet_vegetarian violation, got %+v", resp.Violations)
	}
	threshold := violationsOfType(resp, "nutrient_threshold")
	if len(threshold) != 1 || threshold[0].NutrientID != "sodium" {
		t.Fatalf("expected the AI threshold violation carried over, got %+v", resp.Violations)
	}
	if len(resp.NutrientResults) != 1 || resp.NutrientResults[0].NutrientID != "sodium" {
		t.Fatalf("expected AI nutrient results carried over, got %+v", resp.NutrientResults)
	}
	foundEvidence := false
	for _, e := range resp.EvidencePath {
		if e.DiseaseID == "nutrient_threshold" {
			foundEvidence = true
		}
	}
	if !foundEvidence {
		t.Fatalf("expected a nutrient_threshold evidence path, got %+v", resp.EvidencePath)
	}
}

func TestAnalyzeMeal_ThresholdSurvivesUnavailableAI(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{
		Status: "REJECTED",
		Violations: []domain.MealViolation{
			kgFailureViolation(foodID.String()),
			sodiumThresholdViolation(foodID.String(), "RISK_LEVEL_CRITICAL"),
		},
		NutrientResults: []domain.NutrientThresholdResult{sodiumNutrientResult(foodID.String(), "CRITICAL")},
	}}
	resp := analyzeWithKg(t, greenRiceRepo(foodID), &domain.User{TDEE: 2000}, kg, foodID)

	if resp.Status != "REJECTED" || resp.Safe || resp.HighestRisk != "CRITICAL" {
		t.Fatalf("expected REJECTED/unsafe/CRITICAL, got status=%s safe=%v highest=%s", resp.Status, resp.Safe, resp.HighestRisk)
	}
	if resp.Score.SafetyScore != 0 || resp.Score.ConstraintScore != 0 {
		t.Fatalf("expected zero safety/constraint scores on REJECTED, got %+v", resp.Score)
	}
	if len(violationsOfType(resp, "nutrient_threshold")) != 1 {
		t.Fatalf("expected the threshold violation carried over, got %+v", resp.Violations)
	}
	if len(violationsOfType(resp, "safety_violation")) != 0 {
		t.Fatalf("the untrusted KG-failure violation must not be carried over, got %+v", resp.Violations)
	}
	if len(resp.NutrientResults) != 1 || resp.NutrientResults[0].Status != "CRITICAL" {
		t.Fatalf("expected AI nutrient results preserved, got %+v", resp.NutrientResults)
	}
}

func TestAnalyzeMeal_ThresholdWarningEscalatesLocalApproval(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{
		Status: "WARNING",
		Violations: []domain.MealViolation{
			kgFailureViolation(foodID.String()),
			sodiumThresholdViolation(foodID.String(), "RISK_LEVEL_MEDIUM"),
		},
		NutrientResults: []domain.NutrientThresholdResult{sodiumNutrientResult(foodID.String(), "WARNING")},
	}}
	resp := analyzeWithKg(t, greenRiceRepo(foodID), &domain.User{TDEE: 2000}, kg, foodID)

	if resp.Status != "WARNING" || resp.Safe {
		t.Fatalf("expected WARNING/unsafe, got status=%s safe=%v", resp.Status, resp.Safe)
	}
	if len(violationsOfType(resp, "nutrient_threshold")) != 1 || len(violationsOfType(resp, "safety_violation")) != 0 {
		t.Fatalf("expected only the threshold violation carried over, got %+v", resp.Violations)
	}
}

// TestAnalyzeMeal_NilRepoSendsAbsentIntake: a usecase without a nutrition
// repository (the delivery-test wiring) must not panic and must send today's
// intake as absent, never as an empty present day.
func TestAnalyzeMeal_NilRepoSendsAbsentIntake(t *testing.T) {
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{Status: "APPROVED"}}
	uc := usecase.NewNutritionUseCase(nil, nil, nil, nil, nil, &mockUserRepo{user: &domain.User{}}, nil, kg, nil)
	if _, err := uc.AnalyzeMeal(context.Background(), uuid.New(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "m", FoodIDs: []string{"shrimp"}},
	}); err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}
	intake := kg.lastReq.Candidate.DailyIntake
	if intake == nil || intake.Present || len(intake.Items) != 0 {
		t.Fatalf("expected present=false with no items, got %+v", intake)
	}
}

// TestAnalyzeMeal_UnavailableAIWithoutThresholdsUnchanged: with no threshold
// data, the discard path returns exactly what the AI-error fallback returns.
func TestAnalyzeMeal_UnavailableAIWithoutThresholdsUnchanged(t *testing.T) {
	foodID := uuid.New()
	unavailable := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{
		Status:     "REJECTED",
		Violations: []domain.MealViolation{kgFailureViolation(foodID.String())},
	}}
	got := analyzeWithKg(t, greenRiceRepo(foodID), &domain.User{TDEE: 2000}, unavailable, foodID)

	failed := &mockKgPort{analyzeMealErr: errors.New("AI AnalyzeMeal failed")}
	want := analyzeWithKg(t, greenRiceRepo(foodID), &domain.User{TDEE: 2000}, failed, foodID)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unavailable-AI response without thresholds changed:\n got %+v\nwant %+v", got, want)
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

	// GetWeeklyAnalytics defaults to UTC when the user has no configured
	// timezone. Pin the fixture keys to that same clock so this test remains
	// deterministic around local-midnight/UTC-day boundaries.
	nowUTC := time.Now().UTC()
	today := nowUTC.Format("2006-01-02")
	yesterday := nowUTC.AddDate(0, 0, -1).Format("2006-01-02")

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

// ─── Plan 10-04: fail-closed medical context ────────────────────────────────

const medicalContextUnknownType = "medical_context_unknown"

var errProfileUnreadable = errors.New("profile read failed")

// analyzeWithUsers runs the usecase AnalyzeMeal for one food with a chosen
// user repository and KG port (kg may be nil: no AI client wired).
func analyzeWithUsers(t *testing.T, repo *mockNutritionRepo, users domain.UserRepository, kg domain.NutritionIntelligencePort, foodID uuid.UUID) *domain.AnalyzeMealResponse {
	t.Helper()
	uc := usecase.NewNutritionUseCase(repo, nil, nil, nil, &mockWorkoutRepo{}, users, nil, kg, nil)
	resp, err := uc.AnalyzeMeal(context.Background(), uuid.New(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "manual-lunch", FoodIDs: []string{foodID.String()}, MealType: "lunch"},
	})
	if err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}
	return resp
}

// assertMedicalFloor checks the WARNING floor: exactly one
// medical_context_unknown violation naming the meal's foods, with evidence.
func assertMedicalFloor(t *testing.T, resp *domain.AnalyzeMealResponse, foodID uuid.UUID) domain.MealViolation {
	t.Helper()
	floor := violationsOfType(resp, medicalContextUnknownType)
	if len(floor) != 1 {
		t.Fatalf("expected exactly one %s violation, got %+v", medicalContextUnknownType, resp.Violations)
	}
	if floor[0].Severity != "WARNING" || !reflect.DeepEqual(floor[0].OffendingFoodIDs, []string{foodID.String()}) {
		t.Fatalf("unexpected floor violation %+v", floor[0])
	}
	evidence := 0
	for _, e := range resp.EvidencePath {
		if e.DiseaseID == medicalContextUnknownType && e.RuleDescription == floor[0].Description {
			evidence++
		}
	}
	if evidence != 1 {
		t.Fatalf("expected one matching evidence path entry, got %+v", resp.EvidencePath)
	}
	return floor[0]
}

func assertUnknownSent(t *testing.T, kg *mockKgPort) {
	t.Helper()
	medical := kg.lastReq.MedicalContext
	if medical == nil || medical.Present || len(medical.DiseaseIDs) != 0 {
		t.Fatalf("expected present=false with no ids, got %+v", medical)
	}
}

func TestAnalyzeMeal_MedicalContextProfileErrorFloorsAIApproval(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{Status: "APPROVED"}}
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), &mockUserRepo{getErr: errProfileUnreadable}, kg, foodID)

	assertUnknownSent(t, kg)
	if resp.Status != "WARNING" || resp.Safe || resp.HighestRisk != "WARNING" {
		t.Fatalf("expected WARNING/unsafe/WARNING, got status=%s safe=%v highest=%s", resp.Status, resp.Safe, resp.HighestRisk)
	}
	if len(resp.Violations) != 1 {
		t.Fatalf("expected only the floor violation, got %+v", resp.Violations)
	}
	assertMedicalFloor(t, resp, foodID)
}

func TestAnalyzeMeal_MedicalContextProfileErrorFloorsCatalogFallback(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{
		Status:     "REJECTED",
		Violations: []domain.MealViolation{kgFailureViolation(foodID.String())},
	}}
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), &mockUserRepo{getErr: errProfileUnreadable}, kg, foodID)

	if resp.Status != "WARNING" {
		t.Fatalf("expected the local APPROVED fallback floored to WARNING, got %s", resp.Status)
	}
	if len(violationsOfType(resp, "safety_violation")) != 0 {
		t.Fatalf("the KG-failure violation must still be discarded, got %+v", resp.Violations)
	}
	assertMedicalFloor(t, resp, foodID)
}

func TestAnalyzeMeal_MedicalContextProfileErrorFloorsAIErrorPath(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealErr: errors.New("AI AnalyzeMeal failed")}
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), &mockUserRepo{getErr: errProfileUnreadable}, kg, foodID)

	if resp.Status != "WARNING" {
		t.Fatalf("expected WARNING on the AI-error path, got %s", resp.Status)
	}
	assertMedicalFloor(t, resp, foodID)
}

func TestAnalyzeMeal_MedicalContextProfileErrorFloorsNoKGPath(t *testing.T) {
	foodID := uuid.New()
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), &mockUserRepo{getErr: errProfileUnreadable}, nil, foodID)

	if resp.Status != "WARNING" {
		t.Fatalf("expected WARNING without a KG client, got %s", resp.Status)
	}
	assertMedicalFloor(t, resp, foodID)
}

func TestAnalyzeMeal_MedicalContextProfileErrorKeepsRejected(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{
		Status: "REJECTED",
		Violations: []domain.MealViolation{{
			ViolationType:    "shellfish_allergy",
			Description:      "Shrimp conflicts with shellfish allergy",
			Severity:         "RISK_LEVEL_CRITICAL",
			OffendingFoodIDs: []string{foodID.String()},
		}},
	}}
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), &mockUserRepo{getErr: errProfileUnreadable}, kg, foodID)

	if resp.Status != "REJECTED" || resp.HighestRisk != "CRITICAL" {
		t.Fatalf("REJECTED must stay REJECTED, got status=%s highest=%s", resp.Status, resp.HighestRisk)
	}
	if len(violationsOfType(resp, "shellfish_allergy")) != 1 {
		t.Fatalf("expected the AI violation kept, got %+v", resp.Violations)
	}
	assertMedicalFloor(t, resp, foodID)
}

func TestAnalyzeMeal_MedicalContextUndecryptableIsUnknown(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{Status: "APPROVED"}}
	users := &mockUserRepo{user: &domain.User{TDEE: 2000, MedicalConditions: "v1:QUJDREVGR0hJSktM"}}
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), users, kg, foodID)

	assertUnknownSent(t, kg)
	if resp.Status != "WARNING" {
		t.Fatalf("expected WARNING for an undecryptable profile, got %s", resp.Status)
	}
	assertMedicalFloor(t, resp, foodID)
}

func TestAnalyzeMeal_MedicalContextNilUserIsUnknown(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{Status: "APPROVED"}}
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), &mockUserRepo{nilUser: true}, kg, foodID)

	assertUnknownSent(t, kg)
	if resp.Status != "WARNING" {
		t.Fatalf("expected WARNING for a nil user, got %s", resp.Status)
	}
	assertMedicalFloor(t, resp, foodID)
}

func TestAnalyzeMeal_MedicalContextKnownProfileUnchanged(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{Status: "APPROVED"}}
	users := &mockUserRepo{user: &domain.User{TDEE: 2000, MedicalConditions: " hypertension ,, ckd "}}
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), users, kg, foodID)

	medical := kg.lastReq.MedicalContext
	if medical == nil || !medical.Present || !reflect.DeepEqual(medical.DiseaseIDs, []string{"hypertension", "ckd"}) {
		t.Fatalf("expected present with [hypertension ckd], got %+v", medical)
	}
	if resp.Status != "APPROVED" {
		t.Fatalf("a readable profile must not be floored, got %s", resp.Status)
	}
	if len(violationsOfType(resp, medicalContextUnknownType)) != 0 {
		t.Fatalf("unexpected floor violation %+v", resp.Violations)
	}
}

// TestAnalyzeMeal_MedicalContextFloorWording: the floor must never look like
// an unavailable AI response to Go itself, and must stay user-overridable.
func TestAnalyzeMeal_MedicalContextFloorWording(t *testing.T) {
	foodID := uuid.New()
	kg := &mockKgPort{analyzeMealResp: &domain.AnalyzeMealResponse{Status: "APPROVED"}}
	resp := analyzeWithUsers(t, greenRiceRepo(foodID), &mockUserRepo{getErr: errProfileUnreadable}, kg, foodID)
	floor := assertMedicalFloor(t, resp, foodID)

	description := strings.ToLower(floor.Description)
	// The substrings mealAnalysisUnavailable treats as a discard signal.
	for _, discard := range []string{"not found in database", "failed to evaluate food", "unavailable"} {
		if strings.Contains(description, discard) {
			t.Fatalf("floor description %q contains discard substring %q", floor.Description, discard)
		}
	}
	if strings.Contains(strings.ToUpper(floor.Severity), "CRITICAL") || resp.HighestRisk == "CRITICAL" {
		t.Fatalf("floor must not read as CRITICAL: severity=%s highest=%s", floor.Severity, resp.HighestRisk)
	}
}
