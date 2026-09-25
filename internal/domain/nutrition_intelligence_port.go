package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrFeatureUnavailable = errors.New("feature unavailable in current API version")

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

	// BatchAnalyzeFoods performs vectorized analysis for multiple foods in a single roundtrip.
	BatchAnalyzeFoods(ctx context.Context, foodIDs []string, diseaseIDs []string) (map[string]BatchFoodMetadata, error)

	// ExplainFood returns the KG reasoning path (EvidenceNodes) for why a food is risky.
	ExplainFood(ctx context.Context, userCtx UserNutritionContext, foodID string) (*FoodExplanation, error)

	GetRecommendations(ctx context.Context, userID uuid.UUID, req *GetRecommendationsRequest) (*GetRecommendationsResponse, error)

	// AnalyzeMeal analyzes a composite meal for safety, macros, and violations.
	AnalyzeMeal(ctx context.Context, req *AnalyzeMealRequest) (*AnalyzeMealResponse, error)

	// Thresholds
	GetThresholdSnapshot(ctx context.Context, diseaseIDs []string) (*ThresholdSnapshot, error)

	// Feedback
	SubmitFoodCorrection(ctx context.Context, req *SubmitFoodCorrectionRequest) error
	SubmitFoodAcceptance(ctx context.Context, req *SubmitFoodAcceptanceRequest) error
	SubmitFoodViewed(ctx context.Context, req *SubmitFoodViewedRequest) error

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
// Now defined in explanation.go

// EvidenceNode represents a single node in the KG reasoning path.
// Now defined in explanation.go

// UserNutritionContext contains the user's health profile that
// Go Backend builds from its own DB and sends to the AI Server.
// The AI Server NEVER queries user data directly.
type UserNutritionContext struct {
	UserID   string
	Age      int
	WeightKg float64
	HeightCm float64
	Gender   string // "MALE", "FEMALE", "OTHER"
	Diseases []UserDisease
	Goal     string // "LOSE_WEIGHT", "MAINTAIN_WEIGHT", "GAIN_WEIGHT", "BUILD_MUSCLE"
	Activity string // "SEDENTARY", "LIGHT", "MODERATE", "ACTIVE", "VERY_ACTIVE"
	BMR      float64
	TDEE     float64
}

// UserDisease represents a disease with its severity for the AI context.
type UserDisease struct {
	ID       string
	Name     string
	Severity string // "MILD", "MODERATE", "SEVERE"
}

// ─── Thresholds Domain ───────────────────────────────────────────────

type NutrientThresholdSnapshot struct {
	NutrientID string  `json:"nutrient_id"`
	WarningMg  float64 `json:"warning_mg"`
	CriticalMg float64 `json:"critical_mg"`
}

type ThresholdSnapshot struct {
	Version     int64                       `json:"version"`
	GeneratedAt int64                       `json:"generated_at"`
	NotModified bool                        `json:"not_modified"`
	Thresholds  []NutrientThresholdSnapshot `json:"thresholds"`
}

// ─── Feedback Domain ─────────────────────────────────────────────────

type SubmitFoodCorrectionRequest struct {
	RequestID            string
	PredictedFoodID      string
	PredictedFoodName    string
	FinalFoodID          string
	FinalFoodName        string
	PredictionConfidence float64
	ImageHash            string
	CreatedAt            int64
}

type SubmitFoodAcceptanceRequest struct {
	RequestID            string
	PredictedFoodID      string
	PredictedFoodName    string
	PredictionConfidence float64
	ImageHash            string
	CreatedAt            int64
}

type SubmitFoodViewedRequest struct {
	RequestID            string
	PredictedFoodID      string
	PredictedFoodName    string
	PredictionConfidence float64
	ImageHash            string
	CreatedAt            int64
}

// FoodFeedbackRequest is the unified DTO for the REST Gateway
type FoodFeedbackRequest struct {
	FoodID               string         `json:"food_id"`
	UserAction           string         `json:"user_action"` // "CORRECTION", "ACCEPTANCE", "VIEWED"
	RequestID            string         `json:"request_id"`
	PredictedFoodID      string         `json:"predicted_food_id"`
	PredictedFoodName    string         `json:"predicted_food_name"`
	FinalFoodID          string         `json:"final_food_id"`
	FinalFoodName        string         `json:"final_food_name"`
	PredictionConfidence float64        `json:"prediction_confidence"`
	ImageHash            string         `json:"image_hash"`
	CreatedAt            int64          `json:"created_at"`
	Metadata             map[string]any `json:"metadata"`
}

// ─── Planner / Meal Validation Domain ────────────────────────────────────────

type CandidateMeal struct {
	MealID         string   `json:"meal_id"`
	FoodIDs        []string `json:"food_ids"`
	MealType       string   `json:"meal_type"`
	Ingredients    []string `json:"ingredients"`
	Categories     []string `json:"categories"`
	ProteinSources []string `json:"protein_sources"`
	// FoodRefs carries resolved names for LLM enrichment prompts — AI_server
	// has no Postgres `foods` catalog of its own, so Go (which already has
	// this loaded to build FoodIDs) passes names through here. Populated by
	// AnalyzeMeal() usecase before the gRPC call, not by request callers.
	FoodRefs []FoodRef `json:"food_refs,omitempty"`
	// FoodMasses carries per-food mass inputs from the client, each source in
	// its own optional field (Phase 10 D-02). Passed through to AI_server
	// as-is; see FoodMassInput.
	FoodMasses []FoodMassInput `json:"food_masses,omitempty"`
	// DailyIntake carries today's logged items for the JWT user so AI_server
	// can evaluate the remaining daily nutrient budget (Phase 10 D-01/D-13).
	// Populated by the AnalyzeMeal usecase from the user's food_logs; never
	// bound from client JSON, because a client-supplied empty day would
	// understate intake (Phase 10 RESEARCH Security V4 / Pitfall 2).
	// nil means "not sent" (the planner path): AI_server reads it as absent.
	DailyIntake *DailyIntakeContext `json:"-"`
}

// FoodMassInput is the mass known for one food, by source. Each source stays
// in its own field and they are never merged into one untagged number
// (Phase 10 D-02). ActualMassSource accepts "cv_estimated" or "measured"
// only; anything else is sent as unspecified and AI_server ignores the actual
// slot. Whether a mass may justify REJECTED is decided by AI_server alone
// (D-08, D-10, D-12), so there is deliberately no authority field here.
//
// Go does not validate or reject mass values: AI_server treats non-finite,
// non-positive or over-5000 g values as absent, and a 400 on these new
// optional fields would change the endpoint's behavior for existing clients.
type FoodMassInput struct {
	FoodID            string   `json:"food_id"`
	ActualMassG       *float64 `json:"actual_mass_g,omitempty"`
	ActualMassSource  string   `json:"actual_mass_source,omitempty"` // cv_estimated | measured
	UserEnteredMassG  *float64 `json:"user_entered_mass_g,omitempty"`
	EstimatedServingG *float64 `json:"estimated_serving_g,omitempty"`
}

// MassResolution reports which mass AI_server used for a food and whether it
// had decision authority (Phase 10 D-08). Response-only.
type MassResolution struct {
	FoodID                  string   `json:"food_id"`
	MassG                   *float64 `json:"mass_g,omitempty"`
	MassSource              string   `json:"mass_source"` // cv_estimated | user_entered | estimated_serving | measured | ""
	HC04Approved            bool     `json:"hc04_approved"`
	DecisionAuthority       bool     `json:"decision_authority"`
	AuthoritativeMassG      *float64 `json:"authoritative_mass_g,omitempty"`
	AuthoritativeMassSource string   `json:"authoritative_mass_source"`
	Note                    string   `json:"note,omitempty"`
}

// ConsumedFoodItem is one food already logged today. QuantityG nil means the
// quantity is unknown; it is never sent as 0.
type ConsumedFoodItem struct {
	FoodID    string   `json:"food_id"`
	QuantityG *float64 `json:"quantity_g,omitempty"`
}

// DailyIntakeContext mirrors the proto message of the same name. Present
// false means today's intake could not be loaded (AI_server treats the
// consumed part as unknown); Present true with no Items is a genuinely empty
// day.
type DailyIntakeContext struct {
	Present      bool               `json:"present"`
	Items        []ConsumedFoodItem `json:"items"`
	ConsumedDate string             `json:"consumed_date"` // YYYY-MM-DD
}

type FoodRef struct {
	FoodID string `json:"food_id"`
	Name   string `json:"name"`
	NameVi string `json:"name_vi"`
}

type AnalyzeMealRequest struct {
	Candidate CandidateMeal `json:"candidate"`
	// MedicalContext carries the JWT user's medical conditions (plan 10-04).
	// The AnalyzeMeal usecase always populates it from the user's Postgres
	// profile and never binds it from client JSON, because a client could
	// drop a condition to avoid warnings (NFR-6). nil means "not sent" (the
	// planner path): AI_server then applies no profile conditions.
	MedicalContext *UserMedicalContext `json:"-"`
}

// UserMedicalContext mirrors the proto message of the same name. Present
// true means DiseaseIDs is the user's complete list of raw profile entries
// (empty means no recorded conditions); Present false means the profile
// could not be read, so conditions are UNKNOWN, never "none". AI_server
// canonicalizes the ids. Server-side only: never bound from client JSON.
type UserMedicalContext struct {
	Present    bool     `json:"-"`
	DiseaseIDs []string `json:"-"`
}

type MealScore struct {
	SafetyScore        float64 `json:"safety_score"`
	MacroScore         float64 `json:"macro_score"`
	MicronutrientScore float64 `json:"micronutrient_score"`
	ConstraintScore    float64 `json:"constraint_score"`
}

type MealViolation struct {
	ViolationType    string   `json:"violation_type"`
	Description      string   `json:"description"`
	Severity         string   `json:"severity"`
	OffendingFoodIDs []string `json:"offending_food_ids"`
	// Set only for violation_type "nutrient_threshold" (Phase 10 D-06).
	NutrientID  string   `json:"nutrient_id,omitempty"`
	ProjectedMg *float64 `json:"projected_mg,omitempty"`
	ThresholdMg *float64 `json:"threshold_mg,omitempty"`
}

// NutrientThresholdResult is AI_server's per-nutrient daily-budget
// evaluation for one meal (Phase 10). Go never computes these; it only
// carries them to the client.
type NutrientThresholdResult struct {
	NutrientID             string   `json:"nutrient_id"`
	Status                 string   `json:"status"`
	ThisMealMg             *float64 `json:"this_meal_mg,omitempty"`
	ConsumedTodayMg        *float64 `json:"consumed_today_mg,omitempty"`
	ProjectedMg            *float64 `json:"projected_mg,omitempty"`
	Complete               bool     `json:"complete"`
	WarningMg              float64  `json:"warning_mg"`
	CriticalMg             float64  `json:"critical_mg"`
	DiseaseOverride        bool     `json:"disease_override"`
	CappedNonAuthoritative bool     `json:"capped_non_authoritative"`
	ContributingFoodIDs    []string `json:"contributing_food_ids"`
	Note                   string   `json:"note,omitempty"`
}

type MealEvidencePath struct {
	DiseaseID       string   `json:"diseaseId"`
	RuleDescription string   `json:"ruleDescription"`
	Nodes           []string `json:"nodes"`
}

type MealIngredientEstimate struct {
	Name    string  `json:"name"`
	WeightG float64 `json:"weight_g"`
}

type MealEnrichment struct {
	DishName              string                   `json:"dish_name"`
	EstimatedTotalWeightG float64                  `json:"estimated_total_weight_g"`
	Ingredients           []MealIngredientEstimate `json:"ingredients"`
	Source                string                   `json:"source"`
	Confidence            float64                  `json:"confidence"`
}

type MealFixImpact struct {
	SafetyDelta   float64 `json:"safety_delta"`
	ProteinDelta  float64 `json:"protein_delta"`
	CaloriesDelta float64 `json:"calories_delta"`
	SodiumDelta   float64 `json:"sodium_delta"`
	SugarDelta    float64 `json:"sugar_delta"`
}

type MealFixSuggestion struct {
	Title       string        `json:"title"`
	Replacement CandidateMeal `json:"replacement"`
	Impact      MealFixImpact `json:"impact"`
}

type AnalyzeMealResponse struct {
	Status           string              `json:"status"` // e.g. "APPROVED", "WARNING", "REJECTED"
	Safe             bool                `json:"safe"`
	RiskLevel        string              `json:"riskLevel"`
	HighestRisk      string              `json:"highestRisk"`
	Score            MealScore           `json:"score"`
	Violations       []MealViolation     `json:"violations"`
	EvidencePath     []MealEvidencePath  `json:"evidencePath"`
	Enrichment       *MealEnrichment     `json:"enrichment,omitempty"`
	Fixes            []MealFixSuggestion `json:"fixes"`
	SafeAlternatives []MealFixSuggestion `json:"safeAlternatives"`
	// NutrientResults and MassResolutions are omitted when AI_server sent
	// none, so pre-Phase-10 responses keep their exact JSON shape.
	NutrientResults []NutrientThresholdResult `json:"nutrientResults,omitempty"`
	MassResolutions []MassResolution          `json:"massResolutions,omitempty"`
}
