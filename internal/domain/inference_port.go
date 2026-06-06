package domain

import "context"

type InferenceResult struct {
	FoodLabel           string
	FoodLabelConfidence float64
	VolumeCm3           float64
	Density             float64
	MassG               float64
	Confidence          float64
}

// InferencePort là interface abstract giao tiếp với AI Service (Dependency Inversion).
type InferencePort interface {
	EstimateVolume(ctx context.Context, imageBytes []byte) (*InferenceResult, error)
}
