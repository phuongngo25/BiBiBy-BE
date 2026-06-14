package infrastructure

import (
	"testing"

	pb "nutrix-backend/internal/infrastructure/grpc/pb/inferencev1"
)

func TestValidateEstimateResponseAcceptsPrimaryMassWithDerivedVolume(t *testing.T) {
	response := &pb.EstimateVolumeResponse{
		FoodLabel:           "pho",
		FoodLabelConfidence: 0.99,
		VolumeConfidence:    0,
		MassG:               340,
		HasMass:             true,
	}

	if err := validateEstimateResponse(response); err != nil {
		t.Fatalf("expected direct mass response to be accepted, got %v", err)
	}
}

func TestValidateEstimateResponseRejectsMissingMass(t *testing.T) {
	response := &pb.EstimateVolumeResponse{
		FoodLabel:           "pho",
		FoodLabelConfidence: 0.99,
	}

	if err := validateEstimateResponse(response); err == nil {
		t.Fatal("expected response without mass to be rejected")
	}
}
