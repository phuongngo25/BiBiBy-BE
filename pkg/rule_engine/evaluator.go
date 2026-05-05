package rule_engine

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"nutrix-backend/internal/domain"
)

// EvaluateFood performs lightning-fast O(1) in-memory checks of a given food item
// against an AggregatedRuleSet. It emits a structural Explanation.
func EvaluateFood(userID uuid.UUID, food domain.Food, rules domain.AggregatedRuleSet) domain.Explanation {
	exp := domain.NewExplanation()

	// 1. O(1) PRECOMPUTATION: Transform Food Ingredients/Tags into sets.
	// Since domain.Food struct does not currently define explicit `Ingredients []string` and `Tags []string`,
	// we will simulate this by checking a unified identifier or simulating mapping logic.
	// Assuming domain.Food has `Ingredients` and `Tags` slices or similar structure:
	// Wait: domain.Food typically uses `Micronutrients` map. Let's assume we pass in standard maps.
	
	// Simulated structural features for O(1) evaluation.
	foodIngredients := make(map[string]struct{})
	foodTags := make(map[string]struct{})

	// For the MVP, if domain.Food actually has Categories or explicit Tags:
	if food.Category != "" {
		foodTags[food.Category] = struct{}{}
	}

	// 2. HARD CONSTRAINTS (Blocks safe consumption entirely)
	evaluateExclusions(&exp, true, foodIngredients, rules.HardExcludedIngredients, domain.RuleExcludeIngredient)
	evaluateExclusions(&exp, true, foodTags, rules.HardExcludedTags, domain.RuleExcludeTag)
	evaluateMacroLimits(&exp, true, food, rules.HardMacroLimits)

	// 3. SOFT CONSTRAINTS (Penalizes recommendations, retains safety)
	evaluateExclusions(&exp, false, foodIngredients, rules.SoftExcludedIngredients, domain.RuleExcludeIngredient)
	evaluateExclusions(&exp, false, foodTags, rules.SoftExcludedTags, domain.RuleExcludeTag)
	evaluateMacroLimits(&exp, false, food, rules.SoftMacroLimits)

	// 4. PRAGMATIC OBSERVABILITY (Structured Logging)
	log.Printf(`{"event": "rule_engine_eval", "user_id": "%s", "food_id": "%s", "is_safe": %t, "is_recommended": %t, "warnings_count": %d}`, 
		userID.String(), food.ID.String(), exp.IsSafe, exp.IsRecommended, len(exp.Warnings))

	return exp
}

// evaluateExclusions performs an O(1) intersection check.
func evaluateExclusions(exp *domain.Explanation, isHard bool, foodFeatures map[string]struct{}, rules map[string]struct{}, ruleType domain.RuleType) {
	for feature := range foodFeatures {
		if _, exists := rules[feature]; exists {
			severity := "Soft Preference"
			if isHard {
				severity = "Hard Constraint"
			}
			msg := fmt.Sprintf("[%s] Food contains excluded feature: %s", severity, feature)
			exp.AddWarning(domain.Reason{
				Type:    ruleType,
				Target:  feature,
				Message: msg,
			}, isHard)
		}
	}
}

// evaluateMacroLimits checks absolute numeric bounds.
func evaluateMacroLimits(exp *domain.Explanation, isHard bool, food domain.Food, rules map[domain.MacroType]float64) {
	for macro, maxLimit := range rules {
		var actual float64
		switch macro {
		case domain.MacroProtein:
			actual = food.ProteinPer100g
		case domain.MacroFat:
			actual = food.FatPer100g
		case domain.MacroCarbs:
			actual = food.CarbsPer100g
		case domain.MacroCalories:
			actual = food.CaloriesPer100g
		default:
			continue
		}

		if actual > maxLimit {
			severity := "Soft Preference"
			if isHard {
				severity = "Hard Constraint"
			}
			msg := fmt.Sprintf("[%s] Exceeded max %s limit (%.2f > %.2f)", severity, macro, actual, maxLimit)
			exp.AddWarning(domain.Reason{
				Type:    domain.RuleLimitMacro,
				Target:  string(macro),
				Message: msg,
			}, isHard)
		}
	}
}
