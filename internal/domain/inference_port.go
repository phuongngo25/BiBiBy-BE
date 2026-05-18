package domain

import "context"

// InferencePort là interface abstract giao tiếp với AI Service (Dependency Inversion).
type InferencePort interface {
	EstimateVolume(ctx context.Context, imageBytes []byte) (float64, error)
}
