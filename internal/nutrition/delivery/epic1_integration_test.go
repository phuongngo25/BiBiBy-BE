package delivery_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/infrastructure/metrics"
	"nutrix-backend/internal/nutrition/delivery"
	"nutrix-backend/internal/nutrition/usecase"
)

// mockUserRepository implements domain.UserRepository
type mockUserRepository struct {
	mock.Mock
}

func (m *mockUserRepository) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) != nil {
		return args.Get(0).(*domain.User), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockUserRepository) UpdateProfile(ctx context.Context, id uuid.UUID, req *domain.UpdateProfileRequest) error {
	return nil
}
func (m *mockUserRepository) SaveRefreshToken(ctx context.Context, rt *domain.RefreshToken) error {
	return nil
}
func (m *mockUserRepository) GetRefreshTokenByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	return nil, nil
}
func (m *mockUserRepository) RevokeRefreshToken(ctx context.Context, oldHash string, replacedByHash *string) error {
	return nil
}
func (m *mockUserRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error { return nil }

// mockKGMock implements domain.NutritionIntelligencePort
type mockKGMock struct {
	mock.Mock
}

func (m *mockKGMock) Ping(ctx context.Context) (*domain.PingStatus, error) { return nil, nil }
func (m *mockKGMock) HealthCheck(ctx context.Context) (*domain.AIHealthStatus, error) {
	return nil, nil
}
func (m *mockKGMock) AnalyzeFood(ctx context.Context, userCtx domain.UserNutritionContext, foodID string) (*domain.FoodAnalysisResult, error) {
	return nil, nil
}
func (m *mockKGMock) BatchAnalyzeFoods(ctx context.Context, foodIDs []string, diseaseIDs []string) (map[string]domain.BatchFoodMetadata, error) {
	return nil, nil
}
func (m *mockKGMock) ExplainFood(ctx context.Context, userCtx domain.UserNutritionContext, foodID string) (*domain.FoodExplanation, error) {
	return nil, nil
}
func (m *mockKGMock) GetRecommendations(ctx context.Context, userID uuid.UUID, req *domain.GetRecommendationsRequest) (*domain.GetRecommendationsResponse, error) {
	return nil, nil
}

func (m *mockKGMock) GetThresholdSnapshot(ctx context.Context, diseaseIDs []string) (*domain.ThresholdSnapshot, error) {
	args := m.Called(ctx, diseaseIDs)
	if args.Get(0) != nil {
		return args.Get(0).(*domain.ThresholdSnapshot), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockKGMock) SubmitFoodCorrection(ctx context.Context, req *domain.SubmitFoodCorrectionRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *mockKGMock) SubmitFoodAcceptance(ctx context.Context, req *domain.SubmitFoodAcceptanceRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *mockKGMock) SubmitFoodViewed(ctx context.Context, req *domain.SubmitFoodViewedRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *mockKGMock) AnalyzeMeal(ctx context.Context, req *domain.AnalyzeMealRequest) (*domain.AnalyzeMealResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) != nil {
		return args.Get(0).(*domain.AnalyzeMealResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockKGMock) Close() error { return nil }

// setupRouter sets up the gin router with mocked dependencies
func setupRouter(kg *mockKGMock, userRepo *mockUserRepository) (*gin.Engine, *mockKGMock, *mockUserRepository) {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	// Inject test userID via middleware
	testUserID := uuid.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", testUserID.String())
		c.Next()
	})

	// Only pass the required mocks. Other repos can be nil for Epic 1 APIs.
	uc := usecase.NewNutritionUseCase(nil, nil, nil, nil, nil, userRepo, nil, kg, nil)

	rg := r.Group("/api/v1")
	delivery.NewNutritionHandler(rg, uc)
	return r, kg, userRepo
}

func TestEpic1A_Thresholds(t *testing.T) {
	// T-A1: GET /api/v1/nutrition/thresholds shape and flow
	// T-A2: UseCase inject disease context (must see Hypertension, Diabetes)
	t.Run("T-A1 and T-A2: Successful Snapshot Retrieval", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		// Setup mock User with diseases
		userRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(&domain.User{
			MedicalConditions: "Hypertension, Diabetes", // The DB value
		}, nil)

		// Expected response from Port
		mockResponse := &domain.ThresholdSnapshot{
			Version: 1,
			Thresholds: []domain.NutrientThresholdSnapshot{
				{NutrientID: "NA", WarningMg: 2000, CriticalMg: 2300},
			},
		}

		// Verify that KG Port is called with exactly ["Hypertension", "Diabetes"]
		kg.On("GetThresholdSnapshot", mock.Anything, []string{"Hypertension", "Diabetes"}).Return(mockResponse, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/nutrition/thresholds", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		kg.AssertExpectations(t)
		userRepo.AssertExpectations(t)

		var resp domain.ThresholdSnapshot
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), resp.Version)
		assert.Len(t, resp.Thresholds, 1)
	})

	// T-A3: Port returns error -> 500
	t.Run("T-A3: Port Returns Error", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		userRepo.On("GetByID", mock.Anything, mock.AnythingOfType("uuid.UUID")).Return(&domain.User{}, nil)
		kg.On("GetThresholdSnapshot", mock.Anything, []string(nil)).Return(nil, errors.New("KG offline"))

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/nutrition/thresholds", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "KG offline")
	})
}

func TestEpic1B_Feedback(t *testing.T) {
	// T-B1: Correction
	t.Run("T-B1: Correction -> Correct RPC", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		kg.On("SubmitFoodCorrection", mock.Anything, mock.MatchedBy(func(req *domain.SubmitFoodCorrectionRequest) bool {
			return req.RequestID == "req-123" &&
				req.PredictedFoodID == "food_123" &&
				req.PredictedFoodName == "Bun Bo Hue" &&
				req.FinalFoodName == "Pho Bo" &&
				req.ImageHash == "img-hash-1" &&
				req.CreatedAt == 1710000000000
		})).Return(nil)

		body := []byte(`{
			"food_id":"food_123",
			"user_action":"CORRECTION",
			"request_id":"req-123",
			"predicted_food_name":"Bun Bo Hue",
			"final_food_name":"Pho Bo",
			"prediction_confidence":0.82,
			"image_hash":"img-hash-1",
			"created_at":1710000000000
		}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/feedback/correction", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		kg.AssertExpectations(t)
	})

	// T-B2: Acceptance
	t.Run("T-B2: Acceptance -> Correct RPC", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		kg.On("SubmitFoodAcceptance", mock.Anything, mock.MatchedBy(func(req *domain.SubmitFoodAcceptanceRequest) bool {
			return req.PredictedFoodID == "food_456" &&
				req.PredictedFoodName == "Com Tam" &&
				req.RequestID == "accept-456"
		})).Return(nil)

		body := []byte(`{
			"food_id":"food_456",
			"user_action":"ACCEPTANCE",
			"metadata":{
				"request_id":"accept-456",
				"predicted_food_name":"Com Tam"
			}
		}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/feedback/acceptance", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		kg.AssertExpectations(t)
	})

	// T-B3: Viewed
	t.Run("T-B3: Viewed -> Correct RPC", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		kg.On("SubmitFoodViewed", mock.Anything, mock.MatchedBy(func(req *domain.SubmitFoodViewedRequest) bool {
			return req.PredictedFoodID == "food_789"
		})).Return(nil)

		body := []byte(`{"food_id":"food_789", "user_action":"VIEWED"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/feedback/viewed", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		kg.AssertExpectations(t)
	})

	// T-B4: Unknown Action -> 400
	t.Run("T-B4: Unknown Action", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		// Send HACK payload to /correction
		body := []byte(`{"food_id":"123", "user_action":"HACK"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/feedback/correction", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "invalid user action in payload")
	})

	// T-B6: Event Observability
	t.Run("T-B6: Event Observability Metric", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		kg.On("SubmitFoodAcceptance", mock.Anything, mock.Anything).Return(nil)

		// Capture current metric value
		before := testutil.ToFloat64(metrics.FeedbackEventsTotal.WithLabelValues("ACCEPTANCE"))

		body := []byte(`{"food_id":"obs_123", "user_action":"ACCEPTANCE"}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/feedback/acceptance", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify metric was incremented
		after := testutil.ToFloat64(metrics.FeedbackEventsTotal.WithLabelValues("ACCEPTANCE"))
		assert.Equal(t, before+1, after, "Metric feedback_received_total should increment by 1")
	})
}

func TestEpic1C_PlannerExplain(t *testing.T) {
	t.Run("T-C1: Planner Explain -> Correct RPC Mapping", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		mockResponse := &domain.AnalyzeMealResponse{
			Status: "APPROVED",
			Score: domain.MealScore{
				SafetyScore: 0.95,
			},
			Violations: []domain.MealViolation{},
			Fixes:      []domain.MealFixSuggestion{},
		}

		kg.On("AnalyzeMeal", mock.Anything, mock.MatchedBy(func(req *domain.AnalyzeMealRequest) bool {
			return req.Candidate.MealID == "meal_001"
		})).Return(mockResponse, nil)

		body := []byte(`{"candidate": {"meal_id": "meal_001", "food_ids": ["f1", "f2"], "meal_type": "lunch", "ingredients": [], "categories": [], "protein_sources": []}}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/planner/explain", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		kg.AssertExpectations(t)

		var resp domain.AnalyzeMealResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "APPROVED", resp.Status)
		assert.Equal(t, 0.95, resp.Score.SafetyScore)
	})
}

func TestPhase4CompositeMealValidationAPI(t *testing.T) {
	t.Run("Shrimp shellfish allergy returns CRITICAL with evidence path", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		kg.On("AnalyzeMeal", mock.Anything, mock.MatchedBy(func(req *domain.AnalyzeMealRequest) bool {
			return len(req.Candidate.FoodIDs) == 1 && req.Candidate.FoodIDs[0] == "shrimp"
		})).Return(&domain.AnalyzeMealResponse{
			Status: "REJECTED",
			Violations: []domain.MealViolation{
				{
					ViolationType:    "shellfish_allergy",
					Description:      "Shrimp conflicts with shellfish allergy",
					Severity:         "RISK_LEVEL_CRITICAL",
					OffendingFoodIDs: []string{"shrimp"},
				},
			},
			Fixes: []domain.MealFixSuggestion{
				{
					Title: "Use chicken instead",
					Replacement: domain.CandidateMeal{
						MealID:   "meal_shrimp",
						FoodIDs:  []string{"chicken"},
						MealType: "lunch",
					},
				},
			},
		}, nil)

		body := []byte(`{"candidate":{"meal_id":"meal_shrimp","food_ids":["shrimp"],"meal_type":"lunch"}}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/meal/validate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp domain.AnalyzeMealResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.False(t, resp.Safe)
		assert.Equal(t, "CRITICAL", resp.RiskLevel)
		assert.Equal(t, "CRITICAL", resp.HighestRisk)
		assert.Len(t, resp.Violations, 1)
		assert.Len(t, resp.EvidencePath, 1)
		assert.Contains(t, resp.EvidencePath[0].Nodes, "shrimp")
		assert.Len(t, resp.SafeAlternatives, 1)
	})

	t.Run("Multiple violations returns highest risk", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		kg.On("AnalyzeMeal", mock.Anything, mock.Anything).Return(&domain.AnalyzeMealResponse{
			Status: "WARNING",
			Violations: []domain.MealViolation{
				{ViolationType: "hypertension", Description: "High sodium", Severity: "WARNING", OffendingFoodIDs: []string{"soup"}},
				{ViolationType: "allergy", Description: "Peanut allergy", Severity: "CRITICAL", OffendingFoodIDs: []string{"peanut"}},
			},
		}, nil)

		body := []byte(`{"candidate":{"meal_id":"meal_multi","food_ids":["soup","peanut"],"meal_type":"dinner"}}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/meal/validate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp domain.AnalyzeMealResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "CRITICAL", resp.HighestRisk)
		assert.Len(t, resp.Violations, 2)
		assert.Len(t, resp.EvidencePath, 2)
	})

	t.Run("Allergy and hypertension returns both violations", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		kg.On("AnalyzeMeal", mock.Anything, mock.Anything).Return(&domain.AnalyzeMealResponse{
			Status: "REJECTED",
			Violations: []domain.MealViolation{
				{ViolationType: "allergy", Description: "Shellfish allergy", Severity: "CRITICAL", OffendingFoodIDs: []string{"shrimp"}},
				{ViolationType: "hypertension", Description: "Sodium exceeds threshold", Severity: "WARNING", OffendingFoodIDs: []string{"fish_sauce"}},
			},
		}, nil)

		body := []byte(`{"candidate":{"meal_id":"meal_combo","food_ids":["shrimp","fish_sauce"],"meal_type":"lunch"}}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/meal/validate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp domain.AnalyzeMealResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "CRITICAL", resp.HighestRisk)
		assert.Len(t, resp.Violations, 2)
		assert.Len(t, resp.EvidencePath, 2)
	})

	t.Run("Safe meal returns SAFE and no critical banner signal", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		kg.On("AnalyzeMeal", mock.Anything, mock.Anything).Return(&domain.AnalyzeMealResponse{
			Status:     "APPROVED",
			Violations: []domain.MealViolation{},
		}, nil)

		body := []byte(`{"candidate":{"meal_id":"meal_safe","food_ids":["rice","chicken"],"meal_type":"lunch"}}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/nutrition/meal/validate", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp domain.AnalyzeMealResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.True(t, resp.Safe)
		assert.Equal(t, "SAFE", resp.RiskLevel)
		assert.Equal(t, "SAFE", resp.HighestRisk)
		assert.Empty(t, resp.Violations)
		assert.Empty(t, resp.EvidencePath)
	})
}

func TestEpic1C_PlannerWeeklyPlan(t *testing.T) {
	t.Run("C2-A: Safety Dominance", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		// Mock AnalyzeMeal for Shrimp -> REJECTED
		kg.On("AnalyzeMeal", mock.Anything, mock.MatchedBy(func(req *domain.AnalyzeMealRequest) bool {
			return len(req.Candidate.FoodIDs) > 0 && req.Candidate.FoodIDs[0] == "Shrimp"
		})).Return(&domain.AnalyzeMealResponse{
			Status: "REJECTED",
		}, nil)

		// Mock AnalyzeMeal for Safe Chicken -> APPROVED
		kg.On("AnalyzeMeal", mock.Anything, mock.MatchedBy(func(req *domain.AnalyzeMealRequest) bool {
			return len(req.Candidate.FoodIDs) > 0 && req.Candidate.FoodIDs[0] == "Safe Chicken"
		})).Return(&domain.AnalyzeMealResponse{
			Status: "APPROVED",
		}, nil)

		body := []byte(`{"goal": "High Protein", "candidate_foods": ["Shrimp", "Safe Chicken"]}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/planner/weekly-plan", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		kg.AssertExpectations(t)

		var resp domain.WeeklyPlanResponseDTO
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		// Plan NEVER contains Shrimp
		assert.Len(t, resp.Meals, 1)
		assert.Equal(t, "Safe Chicken", resp.Meals[0].FoodIDs[0])
		assert.Equal(t, "APPROVED", resp.Meals[0].Status)
	})

	t.Run("C2-B: Fail Closed (All Rejected)", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		// Mock AnalyzeMeal for Poison -> REJECTED
		kg.On("AnalyzeMeal", mock.Anything, mock.Anything).Return(&domain.AnalyzeMealResponse{
			Status: "REJECTED",
		}, nil)

		body := []byte(`{"goal": "High Protein", "candidate_foods": ["Poison Apple", "Poison Berry"]}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/planner/weekly-plan", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		// Fail Closed expects 422 Unprocessable Entity
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		kg.AssertExpectations(t)
	})

	t.Run("C2-C: Observability", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		// Trigger an error to check failure metric
		kg.On("AnalyzeMeal", mock.Anything, mock.Anything).Return((*domain.AnalyzeMealResponse)(nil), errors.New("kg crash"))

		body := []byte(`{"goal": "High Protein", "candidate_foods": ["Chicken"]}`)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/planner/weekly-plan", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		// Assert metrics
		failuresMetric := testutil.ToFloat64(metrics.PlannerGenerationFailuresTotal.WithLabelValues("kg_error"))
		assert.GreaterOrEqual(t, failuresMetric, float64(1), "failure metric should increase")

		// Duration metric
		durationMetric := testutil.CollectAndCount(metrics.PlannerGenerationDurationMs)
		assert.GreaterOrEqual(t, durationMetric, 1, "duration metric should be recorded")
	})
}

func TestEpic1C_PlannerReoptimize(t *testing.T) {
	t.Run("C2.5-A: Locality Preservation", func(t *testing.T) {
		kg := new(mockKGMock)
		userRepo := new(mockUserRepository)
		router, _, _ := setupRouter(kg, userRepo)

		// Create a mock 7-day plan (we just test Monday & Tuesday to represent the whole week)
		initialMeals := []domain.PlannedMealDTO{
			{MealID: "m1", Date: "Monday", MealType: "breakfast", FoodIDs: []string{"Oatmeal"}, Status: "APPROVED"},
			{MealID: "m2", Date: "Monday", MealType: "lunch", FoodIDs: []string{"Salad"}, Status: "APPROVED"},
			{MealID: "m3", Date: "Monday", MealType: "dinner", FoodIDs: []string{"Chicken"}, Status: "APPROVED"},
			{MealID: "m4", Date: "Tuesday", MealType: "breakfast", FoodIDs: []string{"Eggs"}, Status: "APPROVED"},
			{MealID: "m5", Date: "Tuesday", MealType: "lunch", FoodIDs: []string{"Pasta"}, Status: "WARNING"},
			{MealID: "m6", Date: "Tuesday", MealType: "dinner", FoodIDs: []string{"Steak"}, Status: "APPROVED"},
			{MealID: "m7", Date: "Wednesday", MealType: "lunch", FoodIDs: []string{"Fish"}, Status: "APPROVED"},
		}

		currentPlan := &domain.WeeklyPlanResponseDTO{
			PlanID: "plan-123",
			Meals:  initialMeals,
		}

		// Mock AnalyzeMeal for the replacement food
		kg.On("AnalyzeMeal", mock.Anything, mock.MatchedBy(func(req *domain.AnalyzeMealRequest) bool {
			return len(req.Candidate.FoodIDs) > 0 && req.Candidate.FoodIDs[0] == "HealthyWrap"
		})).Return(&domain.AnalyzeMealResponse{
			Status: "APPROVED",
		}, nil)

		// The adjustment payload
		reqPayload := domain.ReoptimizeWeeklyPlanRequest{
			PlanID: "plan-123",
			Adjustment: domain.PlannerAdjustmentDTO{
				Type:            "swap_meal",
				TargetDate:      "Tuesday",
				TargetMealType:  "lunch",
				PreferredFoodID: "HealthyWrap",
			},
			CurrentPlan: currentPlan,
		}

		body, _ := json.Marshal(reqPayload)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/planner/reoptimize", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		kg.AssertExpectations(t)

		var resp domain.WeeklyPlanResponseDTO
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)

		// Assert Locality Preservation
		assert.Len(t, resp.Meals, 7, "Should preserve all 7 meals")

		// Monday unchanged
		assert.Equal(t, "Oatmeal", resp.Meals[0].FoodIDs[0])
		assert.Equal(t, "Salad", resp.Meals[1].FoodIDs[0])

		// Tuesday breakfast unchanged
		assert.Equal(t, "Eggs", resp.Meals[3].FoodIDs[0])

		// Tuesday lunch CHANGED
		assert.Equal(t, "Tuesday", resp.Meals[4].Date)
		assert.Equal(t, "lunch", resp.Meals[4].MealType)
		assert.Equal(t, "HealthyWrap", resp.Meals[4].FoodIDs[0], "Tuesday Lunch should be swapped")
		assert.Equal(t, "APPROVED", resp.Meals[4].Status)

		// Tuesday dinner unchanged
		assert.Equal(t, "Steak", resp.Meals[5].FoodIDs[0])

		// Wednesday unchanged
		assert.Equal(t, "Fish", resp.Meals[6].FoodIDs[0])
	})
}
