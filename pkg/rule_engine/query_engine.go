package rule_engine

import (
	"gorm.io/gorm"
	"nutrix-backend/internal/domain"
)

// QueryBuilder defines an abstract interface for database filtering.
// Notice it returns an empty interface{} to prevent ORM leakage (e.g. leaking *gorm.DB).
type QueryBuilder interface {
	Apply(rules domain.AggregatedRuleSet) interface{}
}

// gormQueryBuilder is the internal implementation of QueryBuilder.
type gormQueryBuilder struct {
	db *gorm.DB
}

// NewGormQueryBuilder initializes the QueryBuilder around a GORM instance.
func NewGormQueryBuilder(db *gorm.DB) QueryBuilder {
	return &gormQueryBuilder{db: db}
}

// Apply iterates over the Hard constraints inside the RuleSet and constructs
// highly optimized BATCH NOT EXISTS statements against the database.
func (q *gormQueryBuilder) Apply(rules domain.AggregatedRuleSet) interface{} {
	db := q.db

	// 1. Batch EXCLUDED INGREDIENTS
	if len(rules.HardExcludedIngredients) > 0 {
		var ids []string
		for id := range rules.HardExcludedIngredients {
			ids = append(ids, id)
		}
		// Uses NOT EXISTS with IN (?) to prevent full table scans and eliminate ORM Many2Many overhead.
		db = db.Where("NOT EXISTS (SELECT 1 FROM food_ingredients fi WHERE fi.food_id = foods.id AND fi.ingredient_id IN ?)", ids)
	}

	// 2. Batch EXCLUDED TAGS
	if len(rules.HardExcludedTags) > 0 {
		var ids []string
		for id := range rules.HardExcludedTags {
			ids = append(ids, id)
		}
		db = db.Where("NOT EXISTS (SELECT 1 FROM food_tags ft WHERE ft.food_id = foods.id AND ft.tag_id IN ?)", ids)
	}

	// 3. Apply Hard MACRO LIMITS
	for macro, maxLimit := range rules.HardMacroLimits {
		switch macro {
		case domain.MacroProtein:
			db = db.Where("protein_per_100g <= ?", maxLimit)
		case domain.MacroFat:
			db = db.Where("fat_per_100g <= ?", maxLimit)
		case domain.MacroCarbs:
			db = db.Where("carbs_per_100g <= ?", maxLimit)
		case domain.MacroCalories:
			db = db.Where("calories_per_100g <= ?", maxLimit)
		}
	}

	// Return the mutated *gorm.DB as an interface{}
	return db
}
