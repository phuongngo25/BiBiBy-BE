package infrastructure_test

import (
	"context"
	"io/ioutil"
	"log"
	"testing"
	"time"

	"nutrix-backend/internal/infrastructure"
)

func TestGrpcAIClient_EstimateVolume(t *testing.T) {
	// Khởi tạo client tới AI server
	client, err := infrastructure.NewGrpcAIClient("localhost:50051")
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
