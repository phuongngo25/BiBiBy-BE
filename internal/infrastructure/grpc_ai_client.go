package infrastructure

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"nutrix-backend/internal/domain"
	pb "nutrix-backend/internal/infrastructure/grpc/pb/inferencev1" // Đường dẫn import code gen
	"nutrix-backend/internal/infrastructure/metrics"
)

type grpcAIClient struct {
	client pb.InferenceServiceClient
	conn   *grpc.ClientConn
}

// NewGrpcAIClient khởi tạo kết nối gRPC tới AI server (VD: localhost:50051).
func NewGrpcAIClient(targetURI string) (domain.InferencePort, error) {
	// Create the client without blocking startup. The gRPC channel will connect
	// lazily on the first request and can recover if the CV service starts later.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, targetURI,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(metrics.UnaryClientInterceptor()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AI microservice at %s: %w", targetURI, err)
	}

	return &grpcAIClient{
		client: pb.NewInferenceServiceClient(conn),
		conn:   conn,
	}, nil
}

// Close giải phóng tài nguyên connection.
func (g *grpcAIClient) Close() error {
	return g.conn.Close()
}

func (g *grpcAIClient) EstimateVolume(ctx context.Context, imageBytes []byte) (*domain.InferenceResult, error) {
	// Set Deadline cứng 30s cho request này như AI team yêu cầu
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &pb.EstimateVolumeRequest{
		ImageData: imageBytes,
	}

	start := time.Now()
	res, err := g.client.EstimateVolume(reqCtx, req)
	if err != nil {
		log.Printf("[gRPC] EstimateVolume failed after %v: %v", time.Since(start), err)
		return nil, fmt.Errorf("ai inference failed: %w", err)
	}

	log.Printf("[gRPC] AI Success | ReqID: %s | Latency: %.2fms | Label: %s (%.2f) | Volume: %.2f cm³ | VolumeConf: %.2f | Mass: %.2fg",
		res.RequestId, res.LatencyMs, res.FoodLabel, res.FoodLabelConfidence, res.VolumeCm3, res.VolumeConfidence, res.MassG)

	if err := validateEstimateResponse(res); err != nil {
		return nil, err
	}

	return &domain.InferenceResult{
		FoodLabel:           res.FoodLabel,
		FoodLabelConfidence: float64(res.FoodLabelConfidence),
		VolumeCm3:           float64(res.VolumeCm3),
		Density:             float64(res.DensityGCm3),
		MassG:               float64(res.MassG),
		Confidence:          float64(res.FoodLabelConfidence),
	}, nil
}

func validateEstimateResponse(res *pb.EstimateVolumeResponse) error {
	// Phase 5 promotes direct mass. Volume confidence is zero when volume is
	// derived from mass/density, so it must not reject a valid mass result.
	if !res.HasMass || res.MassG <= 0 {
		return fmt.Errorf("ai did not return a valid mass estimate")
	}
	if res.FoodLabel == "" || res.FoodLabel == "unknown" || res.FoodLabelConfidence < 0.5 {
		return fmt.Errorf("ai food classification confidence too low: %.2f", res.FoodLabelConfidence)
	}
	return nil
}

func (g *grpcAIClient) AnalyzeMealImage(ctx context.Context, imageBytes []byte, userDiseases []string) (*domain.AnalyzeMealResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Note: AnalyzeMealImageRequest in proto uses upload_session_id.
	// For backward compatibility or internal dev, we might need a version that takes bytes,
	// but here we must follow the proto definition.
	req := &pb.AnalyzeMealImageRequest{
		UploadSessionId: "legacy-direct-upload",
		UserDiseases:    userDiseases,
	}

	start := time.Now()
	res, err := g.client.AnalyzeMealImage(reqCtx, req)
	if err != nil {
		st, ok := status.FromError(err)
		log.Printf("[gRPC] AnalyzeMealImage failed after %v: [%v] %v", time.Since(start), st.Code(), st.Message())
		if ok {
			switch st.Code() {
			case codes.InvalidArgument:
				return nil, fmt.Errorf("invalid argument: %s", st.Message())
			case codes.NotFound:
				return nil, fmt.Errorf("resource not found: %s", st.Message())
			case codes.Unavailable:
				return nil, fmt.Errorf("ai service unavailable: %s", st.Message())
			case codes.DeadlineExceeded:
				return nil, fmt.Errorf("ai service timeout: %s", st.Message())
			}
		}
		return nil, fmt.Errorf("ai inference failed: %w", err)
	}

	log.Printf("[gRPC] AI Success | ReqID: %s | Latency: %.2fms | Label: %s | Safe: %v",
		res.RequestId, res.LatencyMs, res.FoodLabel, res.Safe)

	violations := make([]domain.Violation, len(res.Violations))
	for i, v := range res.Violations {
		violations[i] = domain.Violation{
			DiseaseID:   v.DiseaseId,
			DiseaseName: v.DiseaseName,
			Severity:    v.Severity,
			Explanation: v.Explanation,
		}
	}

	recommendations := make([]domain.Recommendation, len(res.Recommendations))
	for i, r := range res.Recommendations {
		reasons := []domain.RecommendationReason{
			{
				ReasonType: r.Reason,
			},
		}
		recommendations[i] = domain.Recommendation{
			FoodID:     r.FoodId,
			FoodName:   r.FoodName,
			MatchScore: float64(r.Score),
			Reasons:    reasons,
		}
	}

	evidencePaths := make([]string, 0, len(res.EvidencePaths))
	for _, ep := range res.EvidencePaths {
		pathStr := ""
		for i, node := range ep.Nodes {
			if i > 0 {
				pathStr += " -> "
			}
			pathStr += node
		}
		evidencePaths = append(evidencePaths, pathStr)
	}

	return &domain.AnalyzeMealResult{
		FoodLabel:           res.FoodLabel,
		FoodLabelConfidence: float64(res.FoodLabelConfidence),
		VolumeCm3:           float64(res.VolumeCm3),
		MassG:               float64(res.MassG),
		Ingredients:         res.Ingredients,
		Safe:                res.Safe,
		RiskLevel:           res.RiskLevel.String(),
		Violations:          violations,
		Recommendations:     recommendations,
		EvidencePaths:       evidencePaths,
	}, nil
}

// BatchAnalyzeFoods calls the NutritionIntelligenceService (Port 50051) via the internal bridge.
// Since grpcAIClient is currently connected to Port 50052 (Inference), this implementation
// assumes a shared connection or expects a distinct client for the Intelligence Service.
// Architect Directive: For monorepo simplicity, we assume the provided connection can resolve
// the BatchAnalyzeFoods RPC if correctly registered.
func (g *grpcAIClient) BatchAnalyzeFoods(ctx context.Context, foodIDs []string, userDiseases []string) (map[string]domain.BatchFoodMetadata, error) {
	// We need to cast the client to the correct interface if they share the same channel
	// However, inferencev1 and intelligencepb are different packages.
	// For Sprint 1A, we'll implement this in a separate client (grpcNutritionClient)
	// as per the existing infrastructure split.
	return nil, fmt.Errorf("use grpcNutritionClient for BatchAnalyzeFoods (Sprint 1A)")
}
