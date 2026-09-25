package usecase_test

import (
	"context"
	"errors"
	"io"
	"net"
	"regexp"
	"sync"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/infrastructure"
	commonpb "nutrix-backend/internal/infrastructure/grpc/pb/commonpb"
	pb "nutrix-backend/internal/infrastructure/grpc/pb/intelligencepb"
	"nutrix-backend/internal/nutrition/usecase"
)

// fakeIntelligenceServer is a loopback AI_server stand-in. It records the
// last AnalyzeMeal request under a mutex (race-free under CI -race) and
// answers with a canned response.
type fakeIntelligenceServer struct {
	pb.UnimplementedNutritionIntelligenceServiceServer

	mu      sync.Mutex
	lastReq *pb.AnalyzeMealRequest
	resp    *pb.AnalyzeMealResponse
}

func (f *fakeIntelligenceServer) AnalyzeMeal(_ context.Context, req *pb.AnalyzeMealRequest) (*pb.AnalyzeMealResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastReq = req
	if f.resp == nil {
		return &pb.AnalyzeMealResponse{Status: pb.MealApprovalStatus_APPROVED}, nil
	}
	return f.resp, nil
}

func (f *fakeIntelligenceServer) received(t *testing.T) *pb.AnalyzeMealRequest {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lastReq == nil {
		t.Fatalf("fake AI server received no AnalyzeMeal request")
	}
	return f.lastReq
}

// startFakeIntelligence serves fake over real gRPC on a loopback port and
// returns the real Go client connected to it.
func startFakeIntelligence(t *testing.T, fake *fakeIntelligenceServer) domain.NutritionIntelligencePort {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	pb.RegisterNutritionIntelligenceServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	client, err := infrastructure.NewGrpcNutritionClient(lis.Addr().String())
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	if closer, ok := client.(io.Closer); ok {
		t.Cleanup(func() { _ = closer.Close() })
	}
	return client
}

func f64(v float64) *float64 { return &v }

// thresholdWireFixture builds a usecase wired to the real gRPC client and a
// fake AI server, with one catalog food that has no local violation.
func thresholdWireFixture(t *testing.T, repo *mockNutritionRepo, fake *fakeIntelligenceServer) (domain.NutritionUseCase, uuid.UUID) {
	t.Helper()
	foodID := uuid.New()
	if repo.foodsByID == nil {
		repo.foodsByID = map[uuid.UUID]domain.Food{}
	}
	repo.foodsByID[foodID] = domain.Food{
		ID:     foodID,
		Name:   "Green rice sweet dessert",
		NameVi: "Che com",
		Source: "VFA_DISH",
	}
	client := startFakeIntelligence(t, fake)
	userRepo := &mockUserRepo{user: &domain.User{TDEE: 2000}}
	uc := usecase.NewNutritionUseCase(repo, nil, nil, nil, &mockWorkoutRepo{}, userRepo, nil, client, nil)
	return uc, foodID
}

func sodiumCriticalResponse(foodID string) *pb.AnalyzeMealResponse {
	return &pb.AnalyzeMealResponse{
		Status: pb.MealApprovalStatus_REJECTED,
		Score:  &pb.MealScore{SafetyScore: 0, MacroScore: 1, MicronutrientScore: 1, ConstraintScore: 0},
		Violations: []*pb.MealViolation{{
			ViolationType:    "nutrient_threshold",
			Description:      "Sodium: projected 1800 mg today meets or exceeds your critical limit of 1500 mg",
			Severity:         commonpb.RiskLevel_RISK_LEVEL_CRITICAL,
			OffendingFoodIds: []string{foodID},
			NutrientId:       "sodium",
			ProjectedMg:      f64(1800),
			ThresholdMg:      f64(1500),
		}},
		NutrientResults: []*pb.NutrientThresholdResult{{
			NutrientId:          "sodium",
			Status:              "CRITICAL",
			ThisMealMg:          f64(1800),
			ProjectedMg:         f64(1800),
			Complete:            true,
			WarningMg:           1000,
			CriticalMg:          1500,
			DiseaseOverride:     true,
			ContributingFoodIds: []string{foodID},
		}},
	}
}

var consumedDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// TestAnalyzeMealThresholdWire_DailyIntakeAndVerdict is the Phase 10 tracer:
// the JWT user's food_logs reach AI_server over real gRPC and the AI's
// nutrient_threshold verdict comes back in the usecase response.
func TestAnalyzeMealThresholdWire_DailyIntakeAndVerdict(t *testing.T) {
	loggedFood := uuid.New()
	repo := &mockNutritionRepo{logs: []domain.MealLog{{FoodID: loggedFood, QuantityGrams: 200}}}
	fake := &fakeIntelligenceServer{}
	uc, foodID := thresholdWireFixture(t, repo, fake)
	fake.resp = sodiumCriticalResponse(foodID.String())

	resp, err := uc.AnalyzeMeal(context.Background(), uuid.New(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "manual-lunch", FoodIDs: []string{foodID.String()}, MealType: "lunch"},
	})
	if err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}

	intake := fake.received(t).GetCandidate().GetDailyIntake()
	if intake == nil || !intake.GetPresent() {
		t.Fatalf("expected present daily intake on the wire, got %+v", intake)
	}
	if len(intake.GetItems()) != 1 {
		t.Fatalf("expected exactly one consumed item, got %+v", intake.GetItems())
	}
	item := intake.GetItems()[0]
	if item.GetFoodId() != loggedFood.String() {
		t.Fatalf("expected consumed food %s, got %s", loggedFood, item.GetFoodId())
	}
	if item.QuantityG == nil || *item.QuantityG != 200 {
		t.Fatalf("expected quantity_g 200 with presence, got %v", item.QuantityG)
	}
	if !consumedDatePattern.MatchString(intake.GetConsumedDate()) {
		t.Fatalf("expected YYYY-MM-DD consumed_date, got %q", intake.GetConsumedDate())
	}

	if resp.Status != "REJECTED" || resp.Safe || resp.HighestRisk != "CRITICAL" {
		t.Fatalf("expected REJECTED/unsafe/CRITICAL, got status=%s safe=%v highest=%s", resp.Status, resp.Safe, resp.HighestRisk)
	}
	if len(resp.Violations) != 1 {
		t.Fatalf("expected one violation, got %+v", resp.Violations)
	}
	v := resp.Violations[0]
	if v.ViolationType != "nutrient_threshold" || v.NutrientID != "sodium" {
		t.Fatalf("unexpected threshold violation: %+v", v)
	}
	if v.ProjectedMg == nil || *v.ProjectedMg != 1800 || v.ThresholdMg == nil || *v.ThresholdMg != 1500 {
		t.Fatalf("expected projected 1800 / threshold 1500, got %v / %v", v.ProjectedMg, v.ThresholdMg)
	}
	if len(resp.NutrientResults) != 1 {
		t.Fatalf("expected one nutrient result, got %+v", resp.NutrientResults)
	}
	r := resp.NutrientResults[0]
	if r.Status != "CRITICAL" || r.ThisMealMg == nil || *r.ThisMealMg != 1800 {
		t.Fatalf("unexpected nutrient result: %+v", r)
	}
}

func TestAnalyzeMealThresholdWire_LogsErrorSendsAbsent(t *testing.T) {
	repo := &mockNutritionRepo{
		logs:         []domain.MealLog{{FoodID: uuid.New(), QuantityGrams: 200}},
		dailyLogsErr: errors.New("db down"),
	}
	fake := &fakeIntelligenceServer{}
	uc, foodID := thresholdWireFixture(t, repo, fake)

	if _, err := uc.AnalyzeMeal(context.Background(), uuid.New(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "m", FoodIDs: []string{foodID.String()}},
	}); err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}

	intake := fake.received(t).GetCandidate().GetDailyIntake()
	if intake == nil {
		t.Fatalf("expected an explicit absent intake context, got nil")
	}
	if intake.GetPresent() || len(intake.GetItems()) != 0 {
		t.Fatalf("expected present=false with no items on repo error, got %+v", intake)
	}
}

func TestAnalyzeMealThresholdWire_EmptyDayIsPresent(t *testing.T) {
	repo := &mockNutritionRepo{}
	fake := &fakeIntelligenceServer{}
	uc, foodID := thresholdWireFixture(t, repo, fake)

	if _, err := uc.AnalyzeMeal(context.Background(), uuid.New(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "m", FoodIDs: []string{foodID.String()}},
	}); err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}

	intake := fake.received(t).GetCandidate().GetDailyIntake()
	if intake == nil || !intake.GetPresent() || len(intake.GetItems()) != 0 {
		t.Fatalf("expected present=true with zero items for an empty day, got %+v", intake)
	}
}

// TestAnalyzeMealThresholdWire_UnsetIntakeStaysUnset covers the planner path:
// a candidate with no DailyIntake must reach AI_server with the field unset.
func TestAnalyzeMealThresholdWire_UnsetIntakeStaysUnset(t *testing.T) {
	fake := &fakeIntelligenceServer{}
	client := startFakeIntelligence(t, fake)

	if _, err := client.AnalyzeMeal(context.Background(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "planner", FoodIDs: []string{uuid.NewString()}},
	}); err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}

	if intake := fake.received(t).GetCandidate().GetDailyIntake(); intake != nil {
		t.Fatalf("expected unset daily intake on the planner path, got %+v", intake)
	}
}
