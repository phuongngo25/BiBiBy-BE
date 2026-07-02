package domain

import "context"

type InferenceResult struct {
	FoodLabel           string
	FoodLabelConfidence float64
	VolumeCm3           float64
	Density             float64
	MassG               float64
	Confidence          float64
	// Predictions holds the classifier's top-K candidates (sorted desc by
	// confidence). Empty for older AI servers that don't populate it.
	Predictions []FoodPrediction
}

// FoodPrediction is one candidate dish from the classifier's top-K distribution.
// MassG is the estimated mass for THIS candidate (label-dependent in legacy
// density mode; identical across candidates in direct-mass mode).
type FoodPrediction struct {
	FoodLabel  string
	Confidence float64
	MassG      float64
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
	// ScanFoodCandidates runs the same classifier as EstimateVolume but returns the
	// raw top-K candidates WITHOUT the strict top-1 confidence gate, so the client
	// can present a picker even when the best guess is below the auto-accept threshold.
	ScanFoodCandidates(ctx context.Context, imageBytes []byte) (*InferenceResult, error)
	AnalyzeMealImage(ctx context.Context, imageBytes []byte, userDiseases []string) (*AnalyzeMealResult, error)
	BatchAnalyzeFoods(ctx context.Context, foodIDs []string, userDiseases []string) (map[string]BatchFoodMetadata, error)
}
