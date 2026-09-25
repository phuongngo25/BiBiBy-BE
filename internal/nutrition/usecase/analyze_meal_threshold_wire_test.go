package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"regexp"
	"strings"
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
	// respond, when non-nil, computes the answer from the request (under mu).
	respond func(*pb.AnalyzeMealRequest) *pb.AnalyzeMealResponse
}

func (f *fakeIntelligenceServer) AnalyzeMeal(_ context.Context, req *pb.AnalyzeMealRequest) (*pb.AnalyzeMealResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastReq = req
	if f.respond != nil {
		return f.respond(req), nil
	}
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
	return thresholdWireFixtureWithUsers(t, repo, fake, &mockUserRepo{user: &domain.User{TDEE: 2000}})
}

// thresholdWireFixtureWithUsers is thresholdWireFixture with a caller-chosen
// user repository (the "Postgres profile" source, plan 10-04).
func thresholdWireFixtureWithUsers(t *testing.T, repo *mockNutritionRepo, fake *fakeIntelligenceServer, userRepo domain.UserRepository) (domain.NutritionUseCase, uuid.UUID) {
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

// TestAnalyzeMealThresholdWire_FoodMassesSeparate: REST food_masses reach
// AI_server as separate optional fields, and mass provenance comes back with
// the authority flags AI_server computed (D-02, D-08).
func TestAnalyzeMealThresholdWire_FoodMassesSeparate(t *testing.T) {
	repo := &mockNutritionRepo{}
	fake := &fakeIntelligenceServer{}
	uc, foodID := thresholdWireFixture(t, repo, fake)
	fake.resp = &pb.AnalyzeMealResponse{
		Status: pb.MealApprovalStatus_APPROVED,
		MassResolutions: []*pb.MassResolution{{
			FoodId:                  foodID.String(),
			MassG:                   f64(180),
			MassSource:              pb.MassSource_MASS_SOURCE_CV_ESTIMATED,
			Hc04Approved:            false,
			DecisionAuthority:       false,
			AuthoritativeMassG:      f64(400),
			AuthoritativeMassSource: pb.MassSource_MASS_SOURCE_USER_ENTERED,
		}},
	}

	resp, err := uc.AnalyzeMeal(context.Background(), uuid.New(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{
			MealID:  "m",
			FoodIDs: []string{foodID.String()},
			FoodMasses: []domain.FoodMassInput{{
				FoodID:           foodID.String(),
				UserEnteredMassG: f64(400),
				ActualMassG:      f64(180),
				ActualMassSource: "cv_estimated",
			}},
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}

	masses := fake.received(t).GetCandidate().GetFoodMasses()
	if len(masses) != 1 {
		t.Fatalf("expected one food mass on the wire, got %+v", masses)
	}
	m := masses[0]
	if m.GetUserEnteredMassG() != 400 || m.GetActualMassG() != 180 {
		t.Fatalf("expected user_entered 400 / actual 180, got %v / %v", m.GetUserEnteredMassG(), m.GetActualMassG())
	}
	if m.ActualMassSource != pb.MassSource_MASS_SOURCE_CV_ESTIMATED {
		t.Fatalf("expected MASS_SOURCE_CV_ESTIMATED, got %v", m.ActualMassSource)
	}
	if m.EstimatedServingG != nil {
		t.Fatalf("expected estimated_serving_g unset, got %v", *m.EstimatedServingG)
	}

	if len(resp.MassResolutions) != 1 {
		t.Fatalf("expected mass resolutions in the usecase response, got %+v", resp.MassResolutions)
	}
	r := resp.MassResolutions[0]
	if r.HC04Approved || r.DecisionAuthority {
		t.Fatalf("expected no HC-04 approval and no decision authority, got %+v", r)
	}
	if r.MassSource != "cv_estimated" || r.AuthoritativeMassSource != "user_entered" {
		t.Fatalf("unexpected mass provenance: %+v", r)
	}
}

// ─── Plan 10-04: medical conditions from the Postgres profile ───────────────

// profileUserRepo stands in for the Postgres users table: GetByID returns
// users[id] (domain.ErrUserNotFound when absent, or err when set) and
// records every requested id.
type profileUserRepo struct {
	mockUserRepo

	mu        sync.Mutex
	users     map[uuid.UUID]*domain.User
	err       error
	requested []uuid.UUID
}

func (r *profileUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requested = append(r.requested, id)
	if r.err != nil {
		return nil, r.err
	}
	user, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return user, nil
}

// distinctRequested returns the requested ids with consecutive repeats
// collapsed (one AnalyzeMeal call may read the same profile more than once).
func (r *profileUserRepo) distinctRequested() []uuid.UUID {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []uuid.UUID
	for _, id := range r.requested {
		if len(out) == 0 || out[len(out)-1] != id {
			out = append(out, id)
		}
	}
	return out
}

// canonicalDiseaseID mirrors AI_server's canonicalize_disease_id.
func canonicalDiseaseID(raw string) string {
	id := strings.ToUpper(strings.TrimSpace(raw))
	if !strings.HasPrefix(id, "D_") {
		id = "D_" + id
	}
	return id
}

// sodiumPolicyResponder stands in for AI_server's threshold policy
// (configs/threshold_policy.yaml sodium, most restrictive wins) for a meal
// whose projected sodium is 1800 mg.
func sodiumPolicyResponder(foodID string) func(*pb.AnalyzeMealRequest) *pb.AnalyzeMealResponse {
	type limits struct{ warning, critical float64 }
	overrides := map[string]limits{
		"D_HYPERTENSION": {1000, 1500},
		"D_CKD":          {800, 1500},
	}
	return func(req *pb.AnalyzeMealRequest) *pb.AnalyzeMealResponse {
		warning, critical, override := 1500.0, 2300.0, false
		if medical := req.GetMedicalContext(); medical.GetPresent() {
			for _, raw := range medical.GetDiseaseIds() {
				if l, ok := overrides[canonicalDiseaseID(raw)]; ok {
					override = true
					warning = math.Min(warning, l.warning)
					critical = math.Min(critical, l.critical)
				}
			}
		}
		const projected = 1800.0
		status, severity, resultStatus := pb.MealApprovalStatus_WARNING, commonpb.RiskLevel_RISK_LEVEL_HIGH, "WARNING"
		threshold := warning
		description := fmt.Sprintf("Sodium: projected %.0f mg today meets or exceeds your warning limit of %.0f mg", projected, warning)
		if projected >= critical {
			status, severity, resultStatus = pb.MealApprovalStatus_REJECTED, commonpb.RiskLevel_RISK_LEVEL_CRITICAL, "CRITICAL"
			threshold = critical
			description = fmt.Sprintf("Sodium: projected %.0f mg today meets or exceeds your critical limit of %.0f mg", projected, critical)
		}
		return &pb.AnalyzeMealResponse{
			Status: status,
			Score:  &pb.MealScore{SafetyScore: 1, MacroScore: 1, MicronutrientScore: 1, ConstraintScore: 1},
			Violations: []*pb.MealViolation{{
				ViolationType:    "nutrient_threshold",
				Description:      description,
				Severity:         severity,
				OffendingFoodIds: []string{foodID},
				NutrientId:       "sodium",
				ProjectedMg:      f64(projected),
				ThresholdMg:      f64(threshold),
			}},
			NutrientResults: []*pb.NutrientThresholdResult{{
				NutrientId:          "sodium",
				Status:              resultStatus,
				ThisMealMg:          f64(projected),
				ProjectedMg:         f64(projected),
				Complete:            true,
				WarningMg:           warning,
				CriticalMg:          critical,
				DiseaseOverride:     override,
				ContributingFoodIds: []string{foodID},
			}},
		}
	}
}

func sodiumResult(t *testing.T, resp *domain.AnalyzeMealResponse) domain.NutrientThresholdResult {
	t.Helper()
	for _, r := range resp.NutrientResults {
		if r.NutrientID == "sodium" {
			return r
		}
	}
	t.Fatalf("no sodium NutrientResult in %+v", resp.NutrientResults)
	return domain.NutrientThresholdResult{}
}

// TestAnalyzeMealThresholdWire_MedicalConditionsFromProfile is the 10-04
// tracer: the JWT user's Postgres medical conditions reach AI_server over
// real gRPC and the disease-specific verdict comes back.
func TestAnalyzeMealThresholdWire_MedicalConditionsFromProfile(t *testing.T) {
	userA, userB := uuid.New(), uuid.New()
	users := &profileUserRepo{users: map[uuid.UUID]*domain.User{
		userA: {ID: userA, TDEE: 2000, MedicalConditions: "hypertension, ckd"},
		userB: {ID: userB, TDEE: 2000, MedicalConditions: ""},
	}}
	fake := &fakeIntelligenceServer{}
	uc, foodID := thresholdWireFixtureWithUsers(t, &mockNutritionRepo{}, fake, users)
	fake.mu.Lock()
	fake.respond = sodiumPolicyResponder(foodID.String())
	fake.mu.Unlock()
	meal := func() *domain.AnalyzeMealRequest {
		return &domain.AnalyzeMealRequest{
			Candidate: domain.CandidateMeal{MealID: "manual-lunch", FoodIDs: []string{foodID.String()}, MealType: "lunch"},
		}
	}

	respA, err := uc.AnalyzeMeal(context.Background(), userA, meal())
	if err != nil {
		t.Fatalf("AnalyzeMeal(A): %v", err)
	}
	medical := fake.received(t).GetMedicalContext()
	if medical == nil || !medical.GetPresent() {
		t.Fatalf("user A: medical_context = %+v, want present", medical)
	}
	if got := medical.GetDiseaseIds(); !reflect.DeepEqual(got, []string{"hypertension", "ckd"}) {
		t.Fatalf("user A: disease_ids = %q, want [hypertension ckd]", got)
	}
	if respA.Status != "REJECTED" || respA.HighestRisk != "CRITICAL" {
		t.Fatalf("user A: status=%s highestRisk=%s, want REJECTED/CRITICAL", respA.Status, respA.HighestRisk)
	}
	sodiumA := sodiumResult(t, respA)
	if !sodiumA.DiseaseOverride || sodiumA.WarningMg != 800 || sodiumA.CriticalMg != 1500 {
		t.Fatalf("user A: sodium result %+v, want override 800/1500", sodiumA)
	}
	if len(respA.Violations) != 1 || respA.Violations[0].ThresholdMg == nil || *respA.Violations[0].ThresholdMg != 1500 {
		t.Fatalf("user A: violations %+v, want one at threshold 1500", respA.Violations)
	}

	respB, err := uc.AnalyzeMeal(context.Background(), userB, meal())
	if err != nil {
		t.Fatalf("AnalyzeMeal(B): %v", err)
	}
	medical = fake.received(t).GetMedicalContext()
	if medical == nil || !medical.GetPresent() || len(medical.GetDiseaseIds()) != 0 {
		t.Fatalf("user B: medical_context = %+v, want present with no ids", medical)
	}
	if respB.Status != "WARNING" {
		t.Fatalf("user B: status=%s, want WARNING", respB.Status)
	}
	sodiumB := sodiumResult(t, respB)
	if sodiumB.DiseaseOverride || sodiumB.CriticalMg != 2300 {
		t.Fatalf("user B: sodium result %+v, want default critical 2300", sodiumB)
	}

	if got := users.distinctRequested(); !reflect.DeepEqual(got, []uuid.UUID{userA, userB}) {
		t.Fatalf("profile lookups = %v, want [A B] (the authenticated user ids)", got)
	}
}

// TestAnalyzeMealThresholdWire_ProfileErrorSendsUnknown: an unreadable
// profile goes over the wire as present=false (never an empty present list)
// and the AI approval comes back floored to WARNING (G-04).
func TestAnalyzeMealThresholdWire_ProfileErrorSendsUnknown(t *testing.T) {
	users := &profileUserRepo{err: errors.New("profile read failed")}
	fake := &fakeIntelligenceServer{}
	uc, foodID := thresholdWireFixtureWithUsers(t, &mockNutritionRepo{}, fake, users)

	resp, err := uc.AnalyzeMeal(context.Background(), uuid.New(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "manual-lunch", FoodIDs: []string{foodID.String()}, MealType: "lunch"},
	})
	if err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}
	medical := fake.received(t).GetMedicalContext()
	if medical == nil || medical.GetPresent() || len(medical.GetDiseaseIds()) != 0 {
		t.Fatalf("medical_context = %+v, want set with present=false and no ids", medical)
	}
	if resp.Status != "WARNING" {
		t.Fatalf("status = %s, want WARNING", resp.Status)
	}
	floor := 0
	for _, v := range resp.Violations {
		if v.ViolationType == "medical_context_unknown" {
			floor++
		}
	}
	if floor != 1 {
		t.Fatalf("expected one medical_context_unknown violation, got %+v", resp.Violations)
	}
}

// TestAnalyzeMealThresholdWire_PlannerPathLeavesMedicalContextUnset: a nil
// MedicalContext (the planner callers) leaves the wire field unset (G-05).
func TestAnalyzeMealThresholdWire_PlannerPathLeavesMedicalContextUnset(t *testing.T) {
	fake := &fakeIntelligenceServer{}
	client := startFakeIntelligence(t, fake)

	if _, err := client.AnalyzeMeal(context.Background(), &domain.AnalyzeMealRequest{
		Candidate: domain.CandidateMeal{MealID: "plan-meal", FoodIDs: []string{"F"}, MealType: "lunch"},
	}); err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}
	if got := fake.received(t).GetMedicalContext(); got != nil {
		t.Fatalf("medical_context = %+v, want unset", got)
	}
}

// TestAnalyzeMealThresholdWire_ClientCannotSupplyConditions (T-10-23): a REST
// body's medical context is never bound, and the usecase sends the profile's.
func TestAnalyzeMealThresholdWire_ClientCannotSupplyConditions(t *testing.T) {
	userID := uuid.New()
	users := &profileUserRepo{users: map[uuid.UUID]*domain.User{
		userID: {ID: userID, TDEE: 2000, MedicalConditions: "hypertension, ckd"},
	}}
	fake := &fakeIntelligenceServer{}
	uc, foodID := thresholdWireFixtureWithUsers(t, &mockNutritionRepo{}, fake, users)

	body := fmt.Sprintf(`{"candidate":{"meal_id":"manual-lunch","food_ids":[%q],"meal_type":"lunch"},`+
		`"medical_context":{"present":true,"disease_ids":[]},`+
		`"MedicalContext":{"Present":true,"DiseaseIDs":[]}}`, foodID.String())
	var req domain.AnalyzeMealRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.MedicalContext != nil {
		t.Fatalf("client medical context must not bind, got %+v", req.MedicalContext)
	}

	if _, err := uc.AnalyzeMeal(context.Background(), userID, &req); err != nil {
		t.Fatalf("AnalyzeMeal: %v", err)
	}
	medical := fake.received(t).GetMedicalContext()
	if medical == nil || !medical.GetPresent() || !reflect.DeepEqual(medical.GetDiseaseIds(), []string{"hypertension", "ckd"}) {
		t.Fatalf("medical_context = %+v, want the profile's [hypertension ckd]", medical)
	}
}
