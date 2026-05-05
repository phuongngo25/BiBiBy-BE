package domain

import "github.com/google/uuid"

// RuleType dictates what aspect of nutrition/food is constrained.
type RuleType string

const (
	RuleExcludeIngredient RuleType = "EXCLUDE_INGREDIENT"
	RuleExcludeTag        RuleType = "EXCLUDE_TAG"
	RuleLimitMacro        RuleType = "LIMIT_MACRO"
)

// MacroType represents trackable macronutrients.
type MacroType string

const (
	MacroCarbs   MacroType = "CARBS"
	MacroProtein MacroType = "PROTEIN"
	MacroFat     MacroType = "FAT"
	MacroCalories MacroType = "CALORIES"
)

// RestrictionRule represents a single constraint evaluated by the Expert System.
type RestrictionRule struct {
	ID          uint      `json:"id"`
	ConditionID string    `json:"condition_id"` // Tracing: Which medical/dietary condition spawned this?
	IsHard      bool      `json:"is_hard"`      // Hard=block (Allergy), Soft=warn (Diet/Preference)
	Type        RuleType  `json:"type"`
	Target      string    `json:"target"`       // e.g. "peanut", or MacroType
	MaxLimit    float64   `json:"max_limit"`    // Used if Type == RuleLimitMacro
}

// AggregatedRuleSet represents the normalized, O(1) optimized constraint blueprint for a user.
type AggregatedRuleSet struct {
	UserID                  uuid.UUID
	SourceConditions        []string

	HardExcludedIngredients map[string]struct{}
	SoftExcludedIngredients map[string]struct{}

	HardExcludedTags        map[string]struct{}
	SoftExcludedTags        map[string]struct{}

	HardMacroLimits         map[MacroType]float64
	SoftMacroLimits         map[MacroType]float64
}

// NewAggregatedRuleSet initializes the maps for a new AggregatedRuleSet.
func NewAggregatedRuleSet(userID uuid.UUID) AggregatedRuleSet {
	return AggregatedRuleSet{
		UserID:                  userID,
		SourceConditions:        make([]string, 0),
		HardExcludedIngredients: make(map[string]struct{}),
		SoftExcludedIngredients: make(map[string]struct{}),
		HardExcludedTags:        make(map[string]struct{}),
		SoftExcludedTags:        make(map[string]struct{}),
		HardMacroLimits:         make(map[MacroType]float64),
		SoftMacroLimits:         make(map[MacroType]float64),
	}
}
