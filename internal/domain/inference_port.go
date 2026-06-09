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

type AnalyzeMealResult struct {
	FoodLabel           string
	FoodLabelConfidence float64
	VolumeCm3           float64
	MassG               float64
	Ingredients         []string
	Safe                bool
	RiskLevel           string
	Violations          []Violation
	Recommendations     []Recommendation
	EvidencePaths       []string
}

type Violation struct {
	DiseaseID   string
	DiseaseName string
	Severity    string
	Explanation string
}

// Recommendation now defined in food.go

type BatchFoodMetadata struct {
	KGMetadata *KGMetadata
}

// InferencePort là interface abstract giao tiếp với AI Service (Dependency Inversion).
type InferencePort interface {
	EstimateVolume(ctx context.Context, imageBytes []byte) (*InferenceResult, error)
	AnalyzeMealImage(ctx context.Context, imageBytes []byte, userDiseases []string) (*AnalyzeMealResult, error)
	BatchAnalyzeFoods(ctx context.Context, foodIDs []string, userDiseases []string) (map[string]BatchFoodMetadata, error)
}
