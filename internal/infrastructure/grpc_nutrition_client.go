package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"nutrix-backend/internal/domain"
	commonpb "nutrix-backend/internal/infrastructure/grpc/pb/commonpb"
	pb "nutrix-backend/internal/infrastructure/grpc/pb/intelligencepb"
	"nutrix-backend/internal/infrastructure/metrics"
)

type grpcNutritionClient struct {
	conn   *grpc.ClientConn
	client pb.NutritionIntelligenceServiceClient
}

// NewGrpcNutritionClient initializes a connection to the Python AI Server (Port 50051).
func NewGrpcNutritionClient(target string) (domain.NutritionIntelligencePort, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI server: %w", err)
	}

	return &grpcNutritionClient{
		conn:   conn,
		client: pb.NewNutritionIntelligenceServiceClient(conn),
	}, nil
}

func (g *grpcNutritionClient) Ping(ctx context.Context) (*domain.PingStatus, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := g.client.HealthCheck(reqCtx, &pb.HealthCheckRequest{
		Meta: &commonpb.RequestMeta{RequestId: generateRequestID()},
	})
	if err != nil {
		return nil, fmt.Errorf("AI Ping failed: %w", err)
	}

	return &domain.PingStatus{
		Status: resp.Status,
	}, nil
}

func (g *grpcNutritionClient) HealthCheck(ctx context.Context) (*domain.AIHealthStatus, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := g.client.HealthCheck(reqCtx, &pb.HealthCheckRequest{
		Meta: &commonpb.RequestMeta{RequestId: generateRequestID()},
	})
	if err != nil {
		return nil, fmt.Errorf("AI HealthCheck failed: %w", err)
	}

	return &domain.AIHealthStatus{
		Status:         resp.Status,
		Version:        resp.Version,
		Neo4jConnected: resp.Neo4JConnected,
		OntologyLoaded: resp.OntologyLoaded,
	}, nil
}

// ─── AnalyzeFood ─────────────────────────────────────────────────

func (g *grpcNutritionClient) AnalyzeFood(ctx context.Context, userCtx domain.UserNutritionContext, foodID string) (*domain.FoodAnalysisResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := g.client.AnalyzeFood(reqCtx, &pb.AnalyzeFoodRequest{
		Meta:   &commonpb.RequestMeta{RequestId: generateRequestID()},
		FoodId: foodID,
	})
	if err != nil {
		return nil, fmt.Errorf("AI AnalyzeFood failed: %w", err)
	}

	// Map proto alternatives to domain
	alternatives := make([]domain.AlternativeFood, 0)

	domainViolations := make([]domain.FoodViolation, len(resp.Violations))
	for i, v := range resp.Violations {
		domainViolations[i] = domain.FoodViolation{
			Description: v,
		}
	}

	return &domain.FoodAnalysisResult{
		FoodID:               resp.FoodId,
		Safe:                 resp.Safe,
		RiskLevel:            resp.RiskLevel.String(),
		Summary:              "",
		Violations:           domainViolations,
		ExplanationAvailable: resp.ExplanationAvailable,
		Alternatives:         alternatives,
	}, nil
}

// ─── BatchAnalyzeFoods ───────────────────────────────────────────

func (g *grpcNutritionClient) BatchAnalyzeFoods(ctx context.Context, foodIDs []string, diseaseIDs []string) (map[string]domain.BatchFoodMetadata, error) {
	metrics.PlannerGenerationFailuresTotal.WithLabelValues("feature_unavailable").Inc()
	return nil, domain.ErrFeatureUnavailable
}

// ─── ExplainFood ─────────────────────────────────────────────────

func (g *grpcNutritionClient) ExplainFood(ctx context.Context, userCtx domain.UserNutritionContext, foodID string) (*domain.FoodExplanation, error) {
	metrics.PlannerGenerationFailuresTotal.WithLabelValues("feature_unavailable").Inc()
	return nil, domain.ErrFeatureUnavailable
}

// ─── GetRecommendations ──────────────────────────────────────────

func (g *grpcNutritionClient) GetRecommendations(ctx context.Context, userID uuid.UUID, req *domain.GetRecommendationsRequest) (*domain.GetRecommendationsResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	protoGaps := make([]*pb.NutrientGapItem, len(req.Gaps))
	for i, gap := range req.Gaps {
		protoGaps[i] = &pb.NutrientGapItem{
			NutrientCode: gap.NutrientCode,
			GapAmount:    float32(gap.GapAmount),
		}
	}

	resp, err := g.client.GetRecommendations(reqCtx, &pb.GetRecommendationsRequest{
		Meta:       &commonpb.RequestMeta{RequestId: generateRequestID()},
		Gaps:       protoGaps,
		DiseaseIds: []string{}, // Medical context can be injected here later
	})
	if err != nil {
		return nil, fmt.Errorf("AI GetRecommendations failed: %w", err)
	}

	domainRecs := make([]domain.Recommendation, len(resp.Recommendations))
	for i, r := range resp.Recommendations {
		reasons := make([]domain.RecommendationReason, len(r.Traces))
		for j, trace := range r.Traces {
			reasons[j] = domain.RecommendationReason{
				NutrientCode:      trace.ReasonCode,
				ContributionScore: float64(trace.Contribution),
				ReasonType:        trace.ReasonType.String(),
			}
		}
		domainRecs[i] = domain.Recommendation{
			FoodID:     r.FoodId,
			FoodName:   r.FoodName,
			MatchScore: float64(r.MatchScore),
			Reasons:    reasons,
		}
	}

	return &domain.GetRecommendationsResponse{
		Recommendations: domainRecs,
	}, nil
}

// ─── Planner ──────────────────────────────────────────────────────

func (g *grpcNutritionClient) AnalyzeMeal(ctx context.Context, req *domain.AnalyzeMealRequest) (*domain.AnalyzeMealResponse, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := g.client.AnalyzeMeal(reqCtx, &pb.AnalyzeMealRequest{
		Meta:      &commonpb.RequestMeta{RequestId: generateRequestID()},
		Candidate: buildPbCandidateMeal(req.Candidate),
	})
	if err != nil {
		return nil, fmt.Errorf("AI AnalyzeMeal failed: %w", err)
	}

	return mapPbAnalyzeMealResponse(resp), nil
}

// buildPbCandidateMeal maps a domain candidate to its wire form. It is pure:
// every optional amount is copied into a fresh pointer, and an unset amount
// stays unset (never 0.0), so AI_server can tell "unknown" from "zero".
func buildPbCandidateMeal(c domain.CandidateMeal) *pb.CandidateMeal {
	foodRefs := make([]*pb.FoodRef, len(c.FoodRefs))
	for i, ref := range c.FoodRefs {
		foodRefs[i] = &pb.FoodRef{FoodId: ref.FoodID, Name: ref.Name, NameVi: ref.NameVi}
	}

	candidate := &pb.CandidateMeal{
		MealId:         c.MealID,
		FoodIds:        c.FoodIDs,
		MealType:       c.MealType,
		Ingredients:    c.Ingredients,
		Categories:     c.Categories,
		ProteinSources: c.ProteinSources,
		FoodRefs:       foodRefs,
	}

	// nil DailyIntake (the planner path) leaves the field unset: AI_server
	// reads "absent" as UNKNOWN consumption, never as an empty day.
	if c.DailyIntake != nil {
		items := make([]*pb.ConsumedFoodItem, len(c.DailyIntake.Items))
		for i, item := range c.DailyIntake.Items {
			items[i] = &pb.ConsumedFoodItem{FoodId: item.FoodID, QuantityG: copyFloat64Ptr(item.QuantityG)}
		}
		candidate.DailyIntake = &pb.DailyIntakeContext{
			Present:      c.DailyIntake.Present,
			Items:        items,
			ConsumedDate: c.DailyIntake.ConsumedDate,
		}
	}

	return candidate
}

// mapPbAnalyzeMealResponse maps the wire response to the domain response.
// It is pure and nil-safe: a nil response maps to an empty, non-nil value.
func mapPbAnalyzeMealResponse(resp *pb.AnalyzeMealResponse) *domain.AnalyzeMealResponse {
	if resp == nil {
		return &domain.AnalyzeMealResponse{}
	}

	violations := make([]domain.MealViolation, len(resp.Violations))
	for i, v := range resp.Violations {
		violations[i] = domain.MealViolation{
			ViolationType:    v.ViolationType,
			Description:      v.Description,
			Severity:         v.Severity.String(),
			OffendingFoodIDs: v.OffendingFoodIds,
			NutrientID:       v.GetNutrientId(),
			ProjectedMg:      copyFloat64Ptr(v.ProjectedMg),
			ThresholdMg:      copyFloat64Ptr(v.ThresholdMg),
		}
	}

	fixes := make([]domain.MealFixSuggestion, len(resp.Fixes))
	for i, f := range resp.Fixes {
		fixes[i] = domain.MealFixSuggestion{
			Title: f.Title,
			Replacement: domain.CandidateMeal{
				MealID:         f.Replacement.MealId,
				FoodIDs:        f.Replacement.FoodIds,
				MealType:       f.Replacement.MealType,
				Ingredients:    f.Replacement.Ingredients,
				Categories:     f.Replacement.Categories,
				ProteinSources: f.Replacement.ProteinSources,
			},
			Impact: domain.MealFixImpact{
				SafetyDelta:   float64(f.Impact.SafetyDelta),
				ProteinDelta:  float64(f.Impact.ProteinDelta),
				CaloriesDelta: float64(f.Impact.CaloriesDelta),
				SodiumDelta:   float64(f.Impact.SodiumDelta),
				SugarDelta:    float64(f.Impact.SugarDelta),
			},
		}
	}

	var score domain.MealScore
	if resp.Score != nil {
		score = domain.MealScore{
			SafetyScore:        float64(resp.Score.SafetyScore),
			MacroScore:         float64(resp.Score.MacroScore),
			MicronutrientScore: float64(resp.Score.MicronutrientScore),
			ConstraintScore:    float64(resp.Score.ConstraintScore),
		}
	}

	var enrichment *domain.MealEnrichment
	if resp.Enrichment != nil {
		ingredients := make([]domain.MealIngredientEstimate, len(resp.Enrichment.Ingredients))
		for i, ing := range resp.Enrichment.Ingredients {
			ingredients[i] = domain.MealIngredientEstimate{
				Name:    ing.Name,
				WeightG: ing.AmountG,
			}
		}
		enrichment = &domain.MealEnrichment{
			DishName:              resp.Enrichment.DishName,
			EstimatedTotalWeightG: resp.Enrichment.EstimatedServingSizeG,
			Ingredients:           ingredients,
			Source:                resp.Enrichment.Source,
			Confidence:            resp.Enrichment.Confidence,
		}
	}

	// Left nil when AI_server sent none, so `omitempty` keeps the
	// pre-Phase-10 JSON shape.
	var nutrientResults []domain.NutrientThresholdResult
	if len(resp.NutrientResults) > 0 {
		nutrientResults = make([]domain.NutrientThresholdResult, 0, len(resp.NutrientResults))
		for _, r := range resp.NutrientResults {
			if r == nil {
				continue
			}
			nutrientResults = append(nutrientResults, domain.NutrientThresholdResult{
				NutrientID:             r.NutrientId,
				Status:                 r.Status,
				ThisMealMg:             copyFloat64Ptr(r.ThisMealMg),
				ConsumedTodayMg:        copyFloat64Ptr(r.ConsumedTodayMg),
				ProjectedMg:            copyFloat64Ptr(r.ProjectedMg),
				Complete:               r.Complete,
				WarningMg:              r.WarningMg,
				CriticalMg:             r.CriticalMg,
				DiseaseOverride:        r.DiseaseOverride,
				CappedNonAuthoritative: r.CappedNonAuthoritative,
				ContributingFoodIDs:    append(make([]string, 0, len(r.ContributingFoodIds)), r.ContributingFoodIds...),
				Note:                   r.Note,
			})
		}
	}

	return &domain.AnalyzeMealResponse{
		Status:          resp.Status.String(),
		Score:           score,
		Violations:      violations,
		Fixes:           fixes,
		Enrichment:      enrichment,
		NutrientResults: nutrientResults,
	}
}

// copyFloat64Ptr returns a fresh pointer to a copy of *v, or nil when v is
// nil, so mapped values never alias their source and unset stays unset.
func copyFloat64Ptr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

// ─── Thresholds ──────────────────────────────────────────────────

func (g *grpcNutritionClient) GetThresholdSnapshot(ctx context.Context, diseaseIDs []string) (*domain.ThresholdSnapshot, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := g.client.GetThresholdSnapshot(reqCtx, &pb.GetThresholdSnapshotRequest{
		Meta:        &commonpb.RequestMeta{RequestId: generateRequestID()},
		DiseaseIds:  diseaseIDs,
		ProfileHash: "", // Will be filled dynamically later
	})
	if err != nil {
		return nil, fmt.Errorf("AI GetThresholdSnapshot failed: %w", err)
	}

	thresholds := make([]domain.NutrientThresholdSnapshot, len(resp.Snapshot.Thresholds))
	for i, t := range resp.Snapshot.Thresholds {
		thresholds[i] = domain.NutrientThresholdSnapshot{
			NutrientID: t.NutrientId,
			WarningMg:  float64(t.WarningMg),
			CriticalMg: float64(t.CriticalMg),
		}
	}

	return &domain.ThresholdSnapshot{
		Version:     resp.Snapshot.Version,
		GeneratedAt: resp.Snapshot.GeneratedAt,
		NotModified: resp.Snapshot.NotModified,
		Thresholds:  thresholds,
	}, nil
}

// ─── Feedback & Corrections ──────────────────────────────────────

func (g *grpcNutritionClient) SubmitFoodCorrection(ctx context.Context, req *domain.SubmitFoodCorrectionRequest) error {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := g.client.SubmitFoodCorrection(reqCtx, &pb.SubmitFoodCorrectionRequest{
		Meta:                 &commonpb.RequestMeta{RequestId: generateRequestID()},
		RequestId:            req.RequestID,
		PredictedFoodName:    req.PredictedFoodName,
		FinalFoodName:        req.FinalFoodName,
		PredictionConfidence: float64(req.PredictionConfidence),
		ImageHash:            req.ImageHash,
		CreatedAt:            req.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("AI SubmitFoodCorrection failed: %w", err)
	}

	return nil
}

func (g *grpcNutritionClient) SubmitFoodAcceptance(ctx context.Context, req *domain.SubmitFoodAcceptanceRequest) error {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := g.client.SubmitFoodCorrection(reqCtx, &pb.SubmitFoodCorrectionRequest{
		Meta:                 &commonpb.RequestMeta{RequestId: generateRequestID()},
		RequestId:            req.RequestID,
		PredictedFoodName:    req.PredictedFoodName,
		FinalFoodName:        req.PredictedFoodName,
		PredictionConfidence: req.PredictionConfidence,
		ImageHash:            req.ImageHash,
		CreatedAt:            req.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("AI SubmitFoodAcceptance fallback failed: %w", err)
	}

	return nil
}

func (g *grpcNutritionClient) SubmitFoodViewed(ctx context.Context, req *domain.SubmitFoodViewedRequest) error {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := g.client.SubmitFoodCorrection(reqCtx, &pb.SubmitFoodCorrectionRequest{
		Meta:                 &commonpb.RequestMeta{RequestId: generateRequestID()},
		RequestId:            req.RequestID,
		PredictedFoodName:    req.PredictedFoodName,
		FinalFoodName:        req.PredictedFoodName,
		PredictionConfidence: req.PredictionConfidence,
		ImageHash:            req.ImageHash,
		CreatedAt:            req.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("AI SubmitFoodViewed fallback failed: %w", err)
	}

	return nil
}

func (g *grpcNutritionClient) Close() error {
	return g.conn.Close()
}

// generateRequestID creates a unique request ID for distributed tracing.
func generateRequestID() string {
	return fmt.Sprintf("go-%d", time.Now().UnixNano())
}
