package rule_engine

import (
	"github.com/google/uuid"
	"nutrix-backend/internal/domain"
)

// NormalizeRules iterates through all user constraint rules and reduces them
// into an optimized O(1) memory structure (AggregatedRuleSet).
// Conflicting limits (e.g. multiple Max Carbs rules) are resolved by enforcing the strictest (minimum) bound.
func NormalizeRules(userID uuid.UUID, rules []domain.RestrictionRule) domain.AggregatedRuleSet {
	agg := domain.NewAggregatedRuleSet(userID)

	for _, rule := range rules {
		// Append to Source Conditions tracing list
		agg.SourceConditions = append(agg.SourceConditions, rule.ConditionID)

		switch rule.Type {
		case domain.RuleExcludeIngredient:
			if rule.IsHard {
				agg.HardExcludedIngredients[rule.Target] = struct{}{}
			} else {
				agg.SoftExcludedIngredients[rule.Target] = struct{}{}
			}

		case domain.RuleExcludeTag:
			if rule.IsHard {
				agg.HardExcludedTags[rule.Target] = struct{}{}
			} else {
				agg.SoftExcludedTags[rule.Target] = struct{}{}
			}

		case domain.RuleLimitMacro:
			macroType := domain.MacroType(rule.Target)
			if rule.IsHard {
				applyStrictestLimit(agg.HardMacroLimits, macroType, rule.MaxLimit)
			} else {
				applyStrictestLimit(agg.SoftMacroLimits, macroType, rule.MaxLimit)
			}
		}
	}

	return agg
}

// applyStrictestLimit modifies the map in place, retaining the lowest possible ceiling limit.
func applyStrictestLimit(limitsMap map[domain.MacroType]float64, macro domain.MacroType, limit float64) {
	if existing, found := limitsMap[macro]; found {
		if limit < existing {
			limitsMap[macro] = limit
		}
	} else {
		limitsMap[macro] = limit
	}
}
