package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/user/usecase"
	"nutrix-backend/pkg/crypto"
)

// ─── Mocks ────────────────────────────────────────────────────────────────────

type mockUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func (m *mockUserRepo) Create(ctx context.Context, u *domain.User) error {
	if m.users == nil {
		m.users = map[uuid.UUID]*domain.User{}
	}
	m.users[u.ID] = u
	return nil
}
func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, domain.ErrUserNotFound
}
func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}
func (m *mockUserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, req *domain.UpdateProfileRequest) error {
	return nil
}

// ── Refresh Token Mock Methods ──
var mockRefreshTokens map[string]*domain.RefreshToken = make(map[string]*domain.RefreshToken)

func (m *mockUserRepo) SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error {
	mockRefreshTokens[rt.TokenHash] = rt
	return nil
}

func (m *mockUserRepo) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	if rt, ok := mockRefreshTokens[hash]; ok {
		return rt, nil
	}
	return nil, domain.ErrInvalidRefreshToken
}

func (m *mockUserRepo) RevokeRefreshToken(ctx context.Context, oldHash string, replacedByHash *string) error {
	if rt, ok := mockRefreshTokens[oldHash]; ok {
		rt.Revoked = true
		rt.ReplacedByTokenHash = replacedByHash
		return nil
	}
	return domain.ErrInvalidRefreshToken
}

func (m *mockUserRepo) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	for _, rt := range mockRefreshTokens {
		if rt.FamilyID == familyID {
			rt.Revoked = true
		}
	}
	return nil
}

type mockDRIRepo struct {
	dri *domain.DRI
	err error
}

func (m *mockDRIRepo) GetByDemographic(ctx context.Context, lifeStage, ageRange string) (*domain.DRI, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.dri, nil
}

// ─── Test Helpers ─────────────────────────────────────────────────────────────

func f64(v float64) *float64 { return &v }

func makeDRI() *domain.DRI {
	return &domain.DRI{
		RdaAi: domain.DRIRequirements{
			Elements: domain.DRIElements{
				CalciumMg:   f64(1000),
				IronMg:      f64(8),
				ZincMg:      f64(11),
				PotassiumMg: f64(3400),
			},
			Vitamins: domain.DRIVitamins{
				VitaminAMcg:   f64(900),
				VitaminCMg:    f64(90),
				VitaminB12Mcg: f64(2.4),
			},
			Macronutrients: domain.DRIMacronutrients{
				CarbohydrateG: f64(130),
				ProteinG:      f64(56),
			},
		},
		Ear: domain.DRIRequirements{
			Macronutrients: domain.DRIMacronutrients{
				ProteinGPerKg: f64(0.66),
			},
		},
	}
}

// mockNutritionRepo returns a fixed set of meal logs.
type mockNutritionRepo struct {
	logs []domain.MealLog
}

func (m *mockNutritionRepo) GetMealLogForUpdate(_ context.Context, _, _ uuid.UUID) (*domain.MealLog, error) {
	return nil, nil
}

func (m *mockNutritionRepo) UpdateMealLog(_ context.Context, _ *domain.MealLog) error {
	return nil
}

func (m *mockNutritionRepo) SearchFoods(_ context.Context, _ string) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepo) SearchFoodsByNutrients(_ context.Context, _, _, _, _ float64) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetFoodByID(_ context.Context, _ uuid.UUID) (*domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetRandomFoods(_ context.Context, _ int) ([]domain.Food, error) {
	return nil, nil
}
func (m *mockNutritionRepo) CreateFood(_ context.Context, _ *domain.Food) error   { return nil }
func (m *mockNutritionRepo) UpsertFoods(_ context.Context, _ []domain.Food) error { return nil }
func (m *mockNutritionRepo) UpdateFoodServingSize(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (m *mockNutritionRepo) LogMeal(_ context.Context, _ *domain.MealLog) error { return nil }
func (m *mockNutritionRepo) GetDailyLogs(_ context.Context, _ uuid.UUID, _ time.Time) ([]domain.MealLog, error) {
	return m.logs, nil
}
func (m *mockNutritionRepo) WithTransaction(_ context.Context, fn func(domain.NutritionRepository) error) error {
	return fn(m)
}
func (m *mockNutritionRepo) GetWeeklyConsumed(_ context.Context, _ uuid.UUID, _ int) (map[string]float64, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetWeeklyBurned(_ context.Context, _ uuid.UUID, _ int) (map[string]float64, error) {
	return nil, nil
}
func (m *mockNutritionRepo) LogWater(_ context.Context, _ *domain.WaterLog) error {
	return nil
}
func (m *mockNutritionRepo) GetDailyConsumedWater(_ context.Context, _ uuid.UUID, _ time.Time) (int, error) {
	return 0, nil
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
	return nil, nil
}
func (m *mockNutritionRepo) GetBurnedRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyCalorieAggregate, error) {
	return nil, nil
}
func (m *mockNutritionRepo) GetWaterRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DailyWaterAggregate, error) {
	return nil, nil
}

type mockWorkoutRepo struct{}

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
	return 0, nil
}

func makeUC(user *domain.User, dri *domain.DRI, driErr error) domain.UserUseCase {
	return makeUCWithLogs(user, dri, driErr, nil)
}

func makeUCWithLogs(user *domain.User, dri *domain.DRI, driErr error, logs []domain.MealLog) domain.UserUseCase {
	userRepo := &mockUserRepo{users: map[uuid.UUID]*domain.User{user.ID: user}}
	driRepo := &mockDRIRepo{dri: dri, err: driErr}
	nutriRepo := &mockNutritionRepo{logs: logs}
	workoutRepo := &mockWorkoutRepo{}
	return usecase.NewUserUseCase(userRepo, driRepo, nutriRepo, workoutRepo, "test-secret", 72)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestRegister_WeakPassword(t *testing.T) {
	uc := makeUC(&domain.User{}, makeDRI(), nil)

	testCases := []string{
		"short1A",       // less than 8 chars
		"nouppercase1",  // no uppercase
		"NOLOWERCASE1",  // no lowercase
		"NoNumbersHere", // no numbers
	}

	for _, pw := range testCases {
		req := &domain.RegisterRequest{
			Username: "testuser",
			Email:    "test@example.com",
			Password: pw,
		}
		_, err := uc.Register(context.Background(), req)
		if !errors.Is(err, domain.ErrWeakPassword) {
			t.Errorf("expected ErrWeakPassword for password %q, got %v", pw, err)
		}
	}
}

func TestRegister_StrongPassword_Success(t *testing.T) {
	uc := makeUC(&domain.User{}, makeDRI(), nil)
	req := &domain.RegisterRequest{
		Username: "stronguser",
		Email:    "strong@example.com",
		Password: "Strong1Password!",
	}
	resp, err := uc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success for strong password, got %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Error("expected both access and refresh tokens")
	}
}

func TestRefreshTokens_SuccessAndRevocation(t *testing.T) {
	userID := uuid.New()
	uc := makeUC(&domain.User{ID: userID, Username: "refreshuser"}, makeDRI(), nil)

	// Create a valid refresh token directly in mock
	validRawToken := "dummy-refresh-token"
	validHash := crypto.HashToken(validRawToken)
	famID := uuid.New()

	mockRefreshTokens[validHash] = &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: validHash,
		FamilyID:  famID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Revoked:   false,
	}

	// Test 1: Refresh should succeed
	req := &domain.RefreshRequest{RefreshToken: validRawToken}
	resp, err := uc.RefreshTokens(context.Background(), req)
	if err != nil {
		t.Fatalf("expected success on refresh, got %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Error("expected new tokens")
	}

	// Test 2: The old token should now be revoked and trigger Reuse Detection!
	_, err = uc.RefreshTokens(context.Background(), req)
	if !errors.Is(err, domain.ErrReuseDetected) {
		t.Fatalf("expected ErrReuseDetected after reusing revoked token, got %v", err)
	}
}

func TestRefreshTokens_Expired(t *testing.T) {
	userID := uuid.New()
	uc := makeUC(&domain.User{ID: userID}, makeDRI(), nil)

	expiredRawToken := "expired-token"
	expiredHash := crypto.HashToken(expiredRawToken)

	mockRefreshTokens[expiredHash] = &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: expiredHash,
		FamilyID:  uuid.New(),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired!
		Revoked:   false,
	}

	req := &domain.RefreshRequest{RefreshToken: expiredRawToken}
	_, err := uc.RefreshTokens(context.Background(), req)
	if !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken for expired token, got %v", err)
	}
}

func TestGetTargets_Success_MaleAdult(t *testing.T) {
	dob := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	user := &domain.User{
		ID:            id,
		Gender:        "male",
		DOB:           &dob,
		WeightKg:      75,
		HeightCm:      175,
		ActivityLevel: "moderate",
		TDEE:          2500,
	}
	uc := makeUC(user, makeDRI(), nil)

	resp, err := uc.GetTargets(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalCalories.Target != 2500 {
		t.Errorf("expected caloric target 2500, got %.1f", resp.TotalCalories.Target)
	}
	if resp.TotalCalories.Current != 0 {
		t.Errorf("expected current=0, got %.1f", resp.TotalCalories.Current)
	}
	if len(resp.Micronutrients) == 0 {
		t.Error("expected micronutrients to be populated")
	}
	if _, ok := resp.Macronutrients["protein"]; !ok {
		t.Error("expected 'protein' key in macronutrients")
	}
}

func TestGetTargets_ProteinOverriddenByDRIBodyWeight(t *testing.T) {
	dob := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	user := &domain.User{
		ID:       id,
		Gender:   "female",
		DOB:      &dob,
		WeightKg: 60,
		TDEE:     1800,
	}
	uc := makeUC(user, makeDRI(), nil)

	resp, err := uc.GetTargets(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// EAR = 0.66 g/kg -> 0.66 * 60 = 39.6
	protein := resp.Macronutrients["protein"].Target
	if protein != 39.6 {
		t.Errorf("expected protein override 39.6g, got %.1f", protein)
	}
}

func TestGetTargets_ProfileIncomplete_NoDOB(t *testing.T) {
	id := uuid.New()
	user := &domain.User{
		ID:     id,
		Gender: "male",
		DOB:    nil, // missing DOB
		TDEE:   2200,
	}
	uc := makeUC(user, makeDRI(), nil)

	// Now expect graceful fallback (age=25, male) rather than an error
	resp, err := uc.GetTargets(context.Background(), id)
	if err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}
	if resp.TotalCalories.Target == 0 {
		t.Error("expected a non-zero calorie target from fallback")
	}
}

func TestGetTargets_ProfileIncomplete_NoGender(t *testing.T) {
	dob := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	user := &domain.User{
		ID:     id,
		Gender: "", // missing gender
		DOB:    &dob,
		TDEE:   2200,
	}
	uc := makeUC(user, makeDRI(), nil)

	// Now expect graceful fallback (gender="male") rather than an error
	resp, err := uc.GetTargets(context.Background(), id)
	if err != nil {
		t.Fatalf("expected graceful fallback, got error: %v", err)
	}
	if resp.TotalCalories.Target == 0 {
		t.Error("expected a non-zero calorie target from fallback")
	}
}

func TestGetTargets_DRINotFound(t *testing.T) {
	dob := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	user := &domain.User{
		ID:     id,
		Gender: "male",
		DOB:    &dob,
	}
	uc := makeUC(user, nil, domain.ErrDRINotFound)

	_, err := uc.GetTargets(context.Background(), id)
	if !errors.Is(err, domain.ErrDRINotFound) {
		t.Errorf("expected ErrDRINotFound, got: %v", err)
	}
}

func TestGetTargets_UserNotFound(t *testing.T) {
	uc := makeUC(&domain.User{ID: uuid.New()}, makeDRI(), nil)
	_, err := uc.GetTargets(context.Background(), uuid.New()) // unknown ID
	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestGetTargets_CalorieFallback_HarrisBenedict(t *testing.T) {
	dob := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	user := &domain.User{
		ID:       id,
		Gender:   "male",
		DOB:      &dob,
		WeightKg: 80,
		HeightCm: 180,
		// No TDEE or WeeklyCalorieBudget set — should compute Harris-Benedict
	}
	uc := makeUC(user, makeDRI(), nil)

	resp, err := uc.GetTargets(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// HB for male 80kg 180cm ~27yr: BMR ~1934, * 1.375 = ~2659 — just verify it's a sensible range
	if resp.TotalCalories.Target < 1500 || resp.TotalCalories.Target > 4000 {
		t.Errorf("unexpected calorie target %.1f — not in sensible range 1500-4000", resp.TotalCalories.Target)
	}
}

func TestMicronutrients_NullValuesSkipped(t *testing.T) {
	dob := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	user := &domain.User{
		ID:       id,
		Gender:   "male",
		DOB:      &dob,
		WeightKg: 75,
		TDEE:     2500,
	}
	// DRI with only calcium set, everything else nil
	sparseDRI := &domain.DRI{
		RdaAi: domain.DRIRequirements{
			Elements: domain.DRIElements{CalciumMg: f64(1000)},
		},
		Ear: domain.DRIRequirements{},
	}
	uc := makeUC(user, sparseDRI, nil)

	resp, err := uc.GetTargets(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Micronutrients) != 1 {
		t.Errorf("expected exactly 1 micro (Calcium), got %d", len(resp.Micronutrients))
	}
	if resp.Micronutrients[0].Name != "Calcium" {
		t.Errorf("expected Calcium, got %s", resp.Micronutrients[0].Name)
	}
}

func TestMicronutrients_AggregationFromLogs(t *testing.T) {
	dob := time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()
	user := &domain.User{
		ID:       id,
		Gender:   "male",
		DOB:      &dob,
		WeightKg: 75,
		TDEE:     2500,
	}

	// Simulate a food with micronutrients stored as "value unit" strings (as seeder saves them)
	food := domain.Food{
		ID:   uuid.New(),
		Name: "Test Food",
		Micronutrients: datatypes.JSONMap{
			"Iron":      "10.800000 mg",
			"Zinc":      "2.800000 mg",
			"Vitamin A": "434.600000 µg",
			"Calcium":   "50.000000 mg",
		},
	}

	// One log: 200g consumed → ratio 2.0
	log := domain.MealLog{
		ID:               uuid.New(),
		UserID:           id,
		FoodID:           food.ID,
		QuantityGrams:    200,
		CaloriesConsumed: 300,
		ProteinConsumed:  15,
		FatConsumed:      10,
		CarbsConsumed:    40,
		Food:             food,
	}

	sparseDRI := &domain.DRI{
		RdaAi: domain.DRIRequirements{
			Elements: domain.DRIElements{
				IronMg:    f64(8),
				ZincMg:    f64(11),
				CalciumMg: f64(1000),
			},
			Vitamins: domain.DRIVitamins{
				VitaminAMcg: f64(900),
			},
		},
		Ear: domain.DRIRequirements{},
	}

	uc := makeUCWithLogs(user, sparseDRI, nil, []domain.MealLog{log})
	resp, err := uc.GetTargets(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Macro Current values
	if resp.TotalCalories.Current != 300 {
		t.Errorf("expected 300 kcal current, got %.1f", resp.TotalCalories.Current)
	}

	// Micronutrient Current: Iron 10.8 * ratio(2.0) = 21.6
	for _, m := range resp.Micronutrients {
		switch m.Name {
		case "Iron":
			if m.Current != 21.6 {
				t.Errorf("Iron: expected 21.6mg current, got %.2f", m.Current)
			}
		case "Zinc":
			if m.Current != 5.6 { // 2.8 * 2
				t.Errorf("Zinc: expected 5.6mg current, got %.2f", m.Current)
			}
		case "Calcium":
			if m.Current != 100.0 { // 50 * 2
				t.Errorf("Calcium: expected 100.0mg current, got %.2f", m.Current)
			}
		}
	}
}
