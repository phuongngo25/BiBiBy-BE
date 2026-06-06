package domain

import "context"

// NutritionIntelligencePort abstracts the gRPC communication with the
// Python AI Server's NutritionIntelligenceService.
//
// Go Backend (Source of Truth for Facts) builds UserContext from its own DB,
// then passes it to the AI Server (Source of Truth for Intelligence) via this port.
//
// Flow: Flutter → Go Backend (JWT verify) → gRPC → AI Server → Neo4j
type NutritionIntelligencePort interface {
	// Ping is a permanent diagnostic endpoint to verify transport connectivity
	Ping(ctx context.Context) (*PingStatus, error)

	// HealthCheck verifies that the AI Server and its dependencies (Neo4j, Ontology) are operational.
	HealthCheck(ctx context.Context) (*AIHealthStatus, error)

	// AnalyzeFood performs a full food analysis in a single roundtrip:
	// safety, risk level, violations, summary, alternatives.
	AnalyzeFood(ctx context.Context, userCtx UserNutritionContext, foodID string) (*FoodAnalysisResult, error)

	// ExplainFood returns the KG reasoning path (EvidenceNodes) for why a food is risky.
	ExplainFood(ctx context.Context, userCtx UserNutritionContext, foodID string) (*FoodExplanation, error)

	// Close releases the gRPC connection.
	Close() error
}

// PingStatus represents the transport connectivity status.
type PingStatus struct {
	Status          string
	ServerVersion   string
	Timestamp       int64
	ContractVersion string
	ContractCommit  string
}

// AIHealthStatus represents the health of the AI Server and its dependencies.
type AIHealthStatus struct {
	Status         string // "healthy", "degraded", "unhealthy"
	Version        string
	Neo4jConnected bool
	OntologyLoaded bool
	DatasetVersion string
}

// FoodAnalysisResult is the domain model returned by AnalyzeFood.
type FoodAnalysisResult struct {
	FoodID               string
	Safe                 bool
	RiskLevel            string // "SAFE", "LOW", "MODERATE", "HIGH", "SEVERE"
	Summary              string
	Violations           []FoodViolation
	ExplanationAvailable bool
	Alternatives         []AlternativeFood
}

// FoodViolation represents a single rule violation.
type FoodViolation struct {
	RuleID      string
	Description string
}

// AlternativeFood represents a safe alternative food suggestion.
type AlternativeFood struct {
	FoodID string
	Name   string
	Reason string
}

// FoodExplanation contains the KG reasoning path for a food.
type FoodExplanation struct {
	FoodID string
	Path   []EvidenceNode
}

// EvidenceNode represents a single node in the KG reasoning path.
type EvidenceNode struct {
	NodeID   string
	NodeType string // "Food", "FoodGroup", "Allergen", "Disease"
	NodeName string
}

// UserNutritionContext contains the user's health profile that
// Go Backend builds from its own DB and sends to the AI Server.
// The AI Server NEVER queries user data directly.
type UserNutritionContext struct {
	UserID    string
	Age       int
	WeightKg  float64
	HeightCm  float64
	Gender    string // "MALE", "FEMALE", "OTHER"
	Diseases  []UserDisease
	Goal      string // "LOSE_WEIGHT", "MAINTAIN_WEIGHT", "GAIN_WEIGHT", "BUILD_MUSCLE"
	Activity  string // "SEDENTARY", "LIGHT", "MODERATE", "ACTIVE", "VERY_ACTIVE"
	BMR       float64
	TDEE      float64
}

// UserDisease represents a disease with its severity for the AI context.
type UserDisease struct {
	ID       string
	Name     string
	Severity string // "MILD", "MODERATE", "SEVERE"
}
