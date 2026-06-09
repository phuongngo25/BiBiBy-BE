package infrastructure_test

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"testing"
	"time"

	"nutrix-backend/internal/infrastructure"
)

func requireAIIntegrationTarget(t *testing.T) string {
	t.Helper()

	if os.Getenv("NUTRIX_RUN_AI_INTEGRATION") != "1" {
		t.Skip("set NUTRIX_RUN_AI_INTEGRATION=1 to run AI gRPC integration tests")
	}

	target := os.Getenv("NUTRIX_AI_GRPC_TARGET")
	if target == "" {
		target = "localhost:50051"
	}
	return target
}

func TestGrpcAIClient_EstimateVolume(t *testing.T) {
	target := requireAIIntegrationTarget(t)

	// Khởi tạo client tới AI server
	client, err := infrastructure.NewGrpcAIClient(target)
	if err != nil {
		t.Fatalf("Failed to create grpc client: %v", err)
	}
	defer func() {
		if closer, ok := client.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	// Đọc ảnh thật (pho.png)
	imageBytes, err := ioutil.ReadFile("../../uploads/pho.png")
	if err != nil {
		t.Fatalf("Failed to read image: %v", err)
	}

	// Gọi EstimateVolume
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := client.EstimateVolume(ctx, imageBytes)
	if err != nil {
		t.Fatalf("EstimateVolume failed: %v", err)
	}

	// Log payload như user yêu cầu
	log.Printf("Integration Spike Payload:")
	log.Printf(" - FoodLabel: %s", result.FoodLabel)
	log.Printf(" - VolumeCm3: %.2f", result.VolumeCm3)
	log.Printf(" - DensityG_Cm3: %.2f", result.Density)
	log.Printf(" - MassG: %.2f", result.MassG)
	log.Printf(" - Confidence: %.2f", result.Confidence)

	if result.MassG == 0 {
		t.Errorf("MassG is 0, which means density mismatch still exists or is unhandled")
	}
}

func TestGrpcAIClient_AnalyzeMealImage(t *testing.T) {
	target := requireAIIntegrationTarget(t)

	client, err := infrastructure.NewGrpcAIClient(target)
	if err != nil {
		t.Fatalf("Failed to create grpc client: %v", err)
	}
	defer func() {
		if closer, ok := client.(interface{ Close() error }); ok {
			closer.Close()
		}
	}()

	imageBytes, err := ioutil.ReadFile("../../uploads/pho.png")
	if err != nil {
		t.Fatalf("Failed to read image: %v", err)
	}

	testCases := []struct {
		name         string
		userDiseases []string
	}{
		{
			name:         "Healthy User",
			userDiseases: []string{},
		},
		{
			name:         "Seafood Allergy User",
			userDiseases: []string{"D_SEAFOOD_ALLERGY"},
		},
		{
			name:         "Diabetes User",
			userDiseases: []string{"D_DIABETES"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			result, err := client.AnalyzeMealImage(ctx, imageBytes, tc.userDiseases)
			if err != nil {
				t.Fatalf("AnalyzeMealImage failed: %v", err)
			}

			log.Printf("--- %s ---", tc.name)
			log.Printf("Request:")
			log.Printf("  Image: pho.png")
			log.Printf("  Diseases: %v", tc.userDiseases)
			log.Printf("Response:")
			log.Printf("  FoodLabel: %s (%.2f)", result.FoodLabel, result.FoodLabelConfidence)
			log.Printf("  VolumeCm3: %.2f | MassG: %.2f", result.VolumeCm3, result.MassG)
			log.Printf("  Ingredients: %v", result.Ingredients)
			log.Printf("  Safe: %v", result.Safe)
			log.Printf("  RiskLevel: %s", result.RiskLevel)
			log.Printf("  Violations: %+v", result.Violations)
			log.Printf("  EvidencePaths: %v", result.EvidencePaths)
			log.Printf("  Recommendations: %+v", result.Recommendations)
			log.Printf("------------------------------------------------")
		})
	}
}
