package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/spoonacular"
)

const cacheHitThreshold = 1

type nutritionUseCase struct {
	repo          domain.NutritionRepository
	spoonClient   *spoonacular.Client
}

// NewNutritionUseCase creates the usecase wired with the nutrition repository and Spoonacular client.
func NewNutritionUseCase(repo domain.NutritionRepository, spoonClient *spoonacular.Client) domain.NutritionUseCase {
	return &nutritionUseCase{repo: repo, spoonClient: spoonClient}
}

// SearchFoods is the main search endpoint used by GET /foods/search?q=keyword.
// It queries local DB first; on cache-miss (< 5 results) it calls Spoonacular
// and caches results. Spoonacular errors are surfaced, not swallowed.
func (u *nutritionUseCase) SearchFoods(ctx context.Context, keyword string) ([]domain.Food, error) {
	if keyword == "" {
		return []domain.Food{}, nil
	}

	// Step 1: local DB lookup
	localResults, _ := u.repo.SearchFoods(ctx, keyword)
	if len(localResults) >= cacheHitThreshold {
		return localResults, nil
	}

	// Step 2: cache-miss → Spoonacular fallback (no allergy filter needed here)
	if u.spoonClient != nil {
		recipes, spErr := u.spoonClient.ComplexSearch(ctx, keyword, "", "", 0)
		if spErr != nil {
			fmt.Printf("[SearchFoods] Spoonacular error for %q: %v\n", keyword, spErr)
			if len(localResults) > 0 {
				return localResults, nil
			}
			return nil, fmt.Errorf("search: local DB empty, Spoonacular unavailable: %w", spErr)
		}
		if len(recipes) > 0 {
			foods := mapComplexResultsToFoods(recipes)
			_ = u.repo.UpsertFoods(ctx, foods)
			return foods, nil
		}
	}

	// Step 3: return local results (may be empty — legitimate case)
	return localResults, nil
}

// SearchSpoonacular implements a health-safe Read-Through Cache:
//   - If diet OR intolerances are specified, bypass local cache entirely (allergy safety).
//   - Otherwise check local DB first; only call API on cache-miss (< 5 results).
func (u *nutritionUseCase) SearchSpoonacular(ctx context.Context, query, diet, intolerances string, maxCarbs int) ([]domain.Food, error) {
	// Health-safety bypass: never serve local results when dietary filters are active.
	if diet == "" && intolerances == "" {
		localResults, err := u.repo.SearchFoods(ctx, query)
		if err == nil && len(localResults) >= cacheHitThreshold {
			return localResults, nil // Cache hit — zero API calls burned
		}
	}

	// Cache miss (or allergy filter present) → call Spoonacular
	recipes, err := u.spoonClient.ComplexSearch(ctx, query, diet, intolerances, maxCarbs)
	if err != nil {
		return nil, fmt.Errorf("spoonacular search failed: %w", err)
	}

	foods := mapComplexResultsToFoods(recipes)

	// Persist to DB (DoNothing on conflict — safe to call even if partially cached)
	_ = u.repo.UpsertFoods(ctx, foods)

	return foods, nil
}

// SearchByNutrients checks local DB with strict numeric bounds before calling API.
func (u *nutritionUseCase) SearchByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]domain.Food, error) {
	localResults, err := u.repo.SearchFoodsByNutrients(ctx, minProtein, maxFat, minCalories, maxCalories)
	if err == nil && len(localResults) >= cacheHitThreshold {
		return localResults, nil
	}

	results, err := u.spoonClient.FindByNutrients(ctx, minProtein, maxFat, minCalories, maxCalories)
	if err != nil {
		return nil, fmt.Errorf("spoonacular findByNutrients failed: %w", err)
	}

	foods := mapNutrientResultsToFoods(results)
	_ = u.repo.UpsertFoods(ctx, foods)
	return foods, nil
}

// SearchByIngredients always calls Spoonacular directly — local DB has no ingredient index.
func (u *nutritionUseCase) SearchByIngredients(ctx context.Context, ingredients string) ([]domain.Food, error) {
	results, err := u.spoonClient.FindByIngredients(ctx, ingredients)
	if err != nil {
		return nil, fmt.Errorf("spoonacular findByIngredients failed: %w", err)
	}

	foods := mapIngredientResultsToFoods(results)
	_ = u.repo.UpsertFoods(ctx, foods)
	return foods, nil
}

// CreateFood builds a Food entity from the request and persists it.
func (u *nutritionUseCase) CreateFood(ctx context.Context, req *domain.CreateFoodRequest) (*domain.Food, error) {
	food := &domain.Food{
		Name:            req.Name,
		Category:        req.Category,
		CaloriesPer100g: req.CaloriesPer100g,
		ProteinPer100g:  req.ProteinPer100g,
		CarbsPer100g:    req.CarbsPer100g,
		FatPer100g:      req.FatPer100g,
		ServingSize:     req.ServingSize,
		Micronutrients:  req.Micronutrients,
		IsVegan:         req.IsVegan,
		ImageURL:        req.ImageURL,
		Source:          "Custom",
	}
	if err := u.repo.CreateFood(ctx, food); err != nil {
		return nil, domain.ErrInternalServerError
	}
	return food, nil
}

// LogMeal verifies the food exists and records it for the user.
func (u *nutritionUseCase) LogMeal(ctx context.Context, userID uuid.UUID, req *domain.LogMealRequest) (*domain.MealLog, error) {
	food, err := u.repo.GetFoodByID(ctx, req.FoodID)
	if err != nil {
		return nil, err
	}

	date, err := time.Parse("2006-01-02", req.ConsumedDate)
	if err != nil {
		date = time.Now()
	}

	ratio := req.QuantityGrams / 100.0

	mealLog := &domain.MealLog{
		UserID:           userID,
		FoodID:           food.ID,
		MealType:         req.MealType,
		QuantityGrams:    req.QuantityGrams,
		ConsumedDate:     date,
		CaloriesConsumed: food.CaloriesPer100g * ratio,
		ProteinConsumed:  food.ProteinPer100g * ratio,
		FatConsumed:      food.FatPer100g * ratio,
		CarbsConsumed:    food.CarbsPer100g * ratio,
	}

	if err := u.repo.LogMeal(ctx, mealLog); err != nil {
		return nil, domain.ErrInternalServerError
	}

	mealLog.Food = *food
	return mealLog, nil
}

// GetDailyPlan builds a summary of today's consumed macros and suggests new foods.
func (u *nutritionUseCase) GetDailyPlan(ctx context.Context, userID uuid.UUID) (*domain.DailyPlanResponse, error) {
	today := time.Now()
	logs, err := u.repo.GetDailyLogs(ctx, userID, today)
	if err != nil {
		return nil, err
	}

	var consumedCalories float64
	for _, log := range logs {
		consumedCalories += log.CaloriesConsumed
	}

	suggestions, err := u.repo.GetRandomFoods(ctx, 3)
	if err != nil {
		suggestions = []domain.Food{}
	}

	return &domain.DailyPlanResponse{
		TargetCalories:   2000.0,
		ConsumedCalories: consumedCalories,
		LoggedMeals:      logs,
		RecommendedFoods: suggestions,
	}, nil
}

// ---------------------------------------------------------------------------
// Mapping helpers — translate Spoonacular DTOs to domain.Food entities
// ---------------------------------------------------------------------------

func mapComplexResultsToFoods(recipes []spoonacular.RecipeResult) []domain.Food {
	foods := make([]domain.Food, 0, len(recipes))
	for _, r := range recipes {
		id := r.ID
		foods = append(foods, domain.Food{
			SpoonacularID:   &id,
			Code:            fmt.Sprintf("SPOON-%d", r.ID), // Unique code prevents empty-string collision
			Name:            r.Title,
			CaloriesPer100g: r.Nutrition.GetNutrient("Calories"),
			ProteinPer100g:  r.Nutrition.GetNutrient("Protein"),
			CarbsPer100g:    r.Nutrition.GetNutrient("Carbohydrates"),
			FatPer100g:      r.Nutrition.GetNutrient("Fat"),
			IsVegan:         r.Vegan,
			IsVegetarian:    r.Vegetarian,
			IsGlutenFree:    r.GlutenFree,
			IsDairyFree:     r.DairyFree,
			ImageURL:        r.Image,
			ServingSize:     fmt.Sprintf("%.0f servings", r.Servings),
			Source:          "Spoonacular",
		})
	}
	return foods
}

func mapNutrientResultsToFoods(results []spoonacular.NutrientSearchResult) []domain.Food {
	foods := make([]domain.Food, 0, len(results))
	for _, r := range results {
		id := r.ID
		foods = append(foods, domain.Food{
			SpoonacularID:   &id,
			Code:            fmt.Sprintf("SPOON-%d", r.ID),
			Name:            r.Title,
			CaloriesPer100g: r.Calories,
			ImageURL:        r.Image,
			Source:          "Spoonacular",
		})
	}
	return foods
}

func mapIngredientResultsToFoods(results []spoonacular.IngredientSearchResult) []domain.Food {
	foods := make([]domain.Food, 0, len(results))
	for _, r := range results {
		id := r.ID
		foods = append(foods, domain.Food{
			SpoonacularID: &id,
			Code:          fmt.Sprintf("SPOON-%d", r.ID),
			Name:          r.Title,
			ImageURL:      fmt.Sprintf("https://spoonacular.com/recipeImages/%s", r.Image),
			Source:        "Spoonacular",
		})
	}
	return foods
}
