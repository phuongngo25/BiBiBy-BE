package infrastructure

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"nutrix-backend/internal/domain"
	commonpb "nutrix-backend/internal/infrastructure/grpc/pb/commonpb"
	pb "nutrix-backend/internal/infrastructure/grpc/pb/intelligencepb"
)

// grpcNutritionClient implements domain.NutritionIntelligencePort
// using gRPC to communicate with the Python AI Server.
type grpcNutritionClient struct {
	client pb.NutritionIntelligenceServiceClient
	conn   *grpc.ClientConn
}

// NewGrpcNutritionClient creates a new gRPC client connected to the AI Server.
// The AI Server trusts the internal network — no JWT verification needed here.
func NewGrpcNutritionClient(targetURI string) (domain.NutritionIntelligencePort, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, targetURI,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI NutritionIntelligence service at %s: %w", targetURI, err)
	}

	log.Printf("[gRPC] Connected to NutritionIntelligence service at %s", targetURI)

	return &grpcNutritionClient{
		client: pb.NewNutritionIntelligenceServiceClient(conn),
		conn:   conn,
	}, nil
}

func (g *grpcNutritionClient) Close() error {
	return g.conn.Close()
}

const (
	ClientContractVersion = "v1.0.1"
	ClientContractCommit  = "b080254"
)

// ─── Diagnostics ─────────────────────────────────────────────────

func (g *grpcNutritionClient) Ping(ctx context.Context) (*domain.PingStatus, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := g.client.Ping(reqCtx, &pb.PingRequest{
		Metadata: &commonpb.RequestMetadata{RequestId: generateRequestID()},
	})
	if err != nil {
		return nil, fmt.Errorf("AI Ping failed: %w", err)
	}

	return &domain.PingStatus{
		Status:          resp.Status,
		ServerVersion:   resp.ServerVersion,
		Timestamp:       resp.Timestamp,
		ContractVersion: resp.ContractVersion,
		ContractCommit:  resp.ContractCommit,
	}, nil
}

// ─── HealthCheck ─────────────────────────────────────────────────

func (g *grpcNutritionClient) HealthCheck(ctx context.Context) (*domain.AIHealthStatus, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := g.client.HealthCheck(reqCtx, &pb.HealthCheckRequest{
		Metadata: &commonpb.RequestMetadata{RequestId: generateRequestID()},
	})
	if err != nil {
		return nil, fmt.Errorf("AI HealthCheck failed: %w", err)
	}

	return &domain.AIHealthStatus{
		Status:         resp.Status,
		Version:        resp.Version,
		Neo4jConnected: resp.Neo4JConnected,
		OntologyLoaded: resp.OntologyLoaded,
		DatasetVersion: resp.DatasetVersion,
	}, nil
}

// ─── AnalyzeFood ─────────────────────────────────────────────────

func (g *grpcNutritionClient) AnalyzeFood(ctx context.Context, userCtx domain.UserNutritionContext, foodID string) (*domain.FoodAnalysisResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	start := time.Now()
	resp, err := g.client.AnalyzeFood(reqCtx, &pb.AnalyzeFoodRequest{
		Metadata: &commonpb.RequestMetadata{RequestId: generateRequestID()},
		User:     buildUserContext(userCtx),
		FoodId:   foodID,
	})
	if err != nil {
		log.Printf("[gRPC] AnalyzeFood failed after %v: %v", time.Since(start), err)
		return nil, fmt.Errorf("AI AnalyzeFood failed: %w", err)
	}

	log.Printf("[gRPC] AnalyzeFood | Food: %s | Safe: %v | Risk: %s | Latency: %v",
		foodID, resp.Safe, resp.RiskLevel.String(), time.Since(start))

	// Map proto violations to domain
	violations := make([]domain.FoodViolation, len(resp.Violations))
	for i, v := range resp.Violations {
		violations[i] = domain.FoodViolation{
			RuleID:      v.RuleId,
			Description: v.Description,
		}
	}

	// Map proto alternatives to domain
	alternatives := make([]domain.AlternativeFood, len(resp.Alternatives))
	for i, a := range resp.Alternatives {
		alternatives[i] = domain.AlternativeFood{
			FoodID: a.FoodId,
			Name:   a.Name,
			Reason: a.Reason,
		}
	}

	return &domain.FoodAnalysisResult{
		FoodID:               resp.FoodId,
		Safe:                 resp.Safe,
		RiskLevel:            resp.RiskLevel.String(),
		Summary:              resp.Summary,
		Violations:           violations,
		ExplanationAvailable: resp.ExplanationAvailable,
		Alternatives:         alternatives,
	}, nil
}

// ─── ExplainFood ─────────────────────────────────────────────────

func (g *grpcNutritionClient) ExplainFood(ctx context.Context, userCtx domain.UserNutritionContext, foodID string) (*domain.FoodExplanation, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := g.client.ExplainFood(reqCtx, &pb.ExplainFoodRequest{
		Metadata: &commonpb.RequestMetadata{RequestId: generateRequestID()},
		User:     buildUserContext(userCtx),
		FoodId:   foodID,
	})
	if err != nil {
		return nil, fmt.Errorf("AI ExplainFood failed: %w", err)
	}

	path := make([]domain.EvidenceNode, len(resp.Path))
	for i, n := range resp.Path {
		path[i] = domain.EvidenceNode{
			NodeID:   n.NodeId,
			NodeType: n.NodeType,
			NodeName: n.NodeName,
		}
	}

	return &domain.FoodExplanation{
		FoodID: resp.FoodId,
		Path:   path,
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────

// buildUserContext converts the domain UserNutritionContext into a protobuf UserContext.
// The Go Backend builds this from its own DB so the AI Server never needs to query user data.
func buildUserContext(u domain.UserNutritionContext) *commonpb.UserContext {
	// Map gender string to proto enum
	genderMap := map[string]commonpb.Gender{
		"MALE":   commonpb.Gender_GENDER_MALE,
		"FEMALE": commonpb.Gender_GENDER_FEMALE,
		"OTHER":  commonpb.Gender_GENDER_OTHER,
	}

	// Map goal string to proto enum
	goalMap := map[string]commonpb.GoalType{
		"LOSE_WEIGHT":     commonpb.GoalType_GOAL_LOSE_WEIGHT,
		"MAINTAIN_WEIGHT": commonpb.GoalType_GOAL_MAINTAIN_WEIGHT,
		"GAIN_WEIGHT":     commonpb.GoalType_GOAL_GAIN_WEIGHT,
		"BUILD_MUSCLE":    commonpb.GoalType_GOAL_BUILD_MUSCLE,
	}

	// Map activity string to proto enum
	activityMap := map[string]commonpb.ActivityLevel{
		"SEDENTARY":   commonpb.ActivityLevel_ACTIVITY_SEDENTARY,
		"LIGHT":       commonpb.ActivityLevel_ACTIVITY_LIGHT,
		"MODERATE":    commonpb.ActivityLevel_ACTIVITY_MODERATE,
		"ACTIVE":      commonpb.ActivityLevel_ACTIVITY_ACTIVE,
		"VERY_ACTIVE": commonpb.ActivityLevel_ACTIVITY_VERY_ACTIVE,
	}

	// Map severity string to proto enum
	severityMap := map[string]commonpb.SeverityLevel{
		"MILD":     commonpb.SeverityLevel_MILD,
		"MODERATE": commonpb.SeverityLevel_MODERATE,
		"SEVERE":   commonpb.SeverityLevel_SEVERE,
	}

	diseases := make([]*commonpb.Disease, len(u.Diseases))
	for i, d := range u.Diseases {
		diseases[i] = &commonpb.Disease{
			Id:       d.ID,
			Name:     d.Name,
			Severity: severityMap[d.Severity],
		}
	}

	return &commonpb.UserContext{
		UserId:   u.UserID,
		Age:      int32(u.Age),
		WeightKg: u.WeightKg,
		HeightCm: u.HeightCm,
		Gender:   genderMap[u.Gender],
		Diseases: diseases,
		Goal:     goalMap[u.Goal],
		Activity: activityMap[u.Activity],
		Bmr:      u.BMR,
		Tdee:     u.TDEE,
	}
}

// generateRequestID creates a unique request ID for distributed tracing.
func generateRequestID() string {
	return fmt.Sprintf("go-%d", time.Now().UnixNano())
}
