package usecase

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/spoonacular"
)

type nutritionUseCase struct {
	repo        domain.NutritionRepository
	spoonClient *spoonacular.Client
	workoutRepo domain.WorkoutRepository
}

// NewNutritionUseCase creates the usecase wired with the nutrition repository and Spoonacular client.
func NewNutritionUseCase(repo domain.NutritionRepository, spoonClient *spoonacular.Client, workoutRepo domain.WorkoutRepository) domain.NutritionUseCase {
	return &nutritionUseCase{repo: repo, spoonClient: spoonClient, workoutRepo: workoutRepo}
}

// SearchFoods is the "Local First, API Second" entrypoint (GET /nutrition/foods/search?q=).
//
// The local DB query uses pg_trgm fuzzy matching + 3-tier relevance ordering,
// so ANY non-empty local result set is considered high-quality and returned immediately.
//
// Spoonacular is ONLY called when localResults is completely empty (true cache miss).
// New remote results are then upserted asynchronously for future local cache hits.
func (u *nutritionUseCase) SearchFoods(ctx context.Context, keyword string) ([]domain.Food, error) {
	if strings.TrimSpace(keyword) == "" {
		return []domain.Food{}, nil
	}

	// ── Local DB (always first) ───────────────────────────────────────────────
	// The repository applies fuzzy + relevance ranking; any result here is trustworthy.
	localResults, _ := u.repo.SearchFoods(ctx, keyword)
	if len(localResults) > 0 {
		log.Printf("[SearchFoods] Local HIT for %q → %d results (no API call)", keyword, len(localResults))
		return localResults, nil
	}

	// ── Spoonacular fallback (true cache miss only) ───────────────────────────
	if u.spoonClient == nil {
		return []domain.Food{}, nil
	}

	log.Printf("[SearchFoods] Local MISS for %q — calling Spoonacular", keyword)

	recipes, spErr := u.spoonClient.ComplexSearch(ctx, keyword, "", "", 0)
	if spErr != nil {
		log.Printf("[SearchFoods] Spoonacular error for %q: %v", keyword, spErr)
		return []domain.Food{}, nil // Degrade gracefully — never 500 the user
	}

	if len(recipes) == 0 {
		return []domain.Food{}, nil
	}

	spoonFoods := mapComplexResultsToFoods(recipes)

	// ── Async upsert — build local cache for future searches ──────────────────
	go func(foods []domain.Food) {
		if err := u.repo.UpsertFoods(context.Background(), foods); err != nil {
			log.Printf("[SearchFoods] Background upsert failed for %q: %v", keyword, err)
		} else {
			log.Printf("[SearchFoods] Background upsert: cached %d foods for %q", len(foods), keyword)
		}
	}(spoonFoods)

	return spoonFoods, nil
}

// SearchSpoonacular provides a health-safe Read-Through Cache.
// If diet OR intolerances are specified, bypass local cache entirely (allergy safety).
// Otherwise, check local DB first and only call API on cache-miss.
func (u *nutritionUseCase) SearchSpoonacular(ctx context.Context, query, diet, intolerances string, maxCarbs int) ([]domain.Food, error) {
	if u.spoonClient == nil {
		return nil, fmt.Errorf("spoonacular client is not configured")
	}

	// Health-safety bypass: never serve potentially mismatched local results when dietary filters are active
	if diet == "" && intolerances == "" {
		localResults, err := u.repo.SearchFoods(ctx, query)
		if err == nil && len(localResults) > 0 {
			return localResults, nil
		}
	}

	// Cache miss (or allergy filter present) → call Spoonacular
	recipes, err := u.spoonClient.ComplexSearch(ctx, query, diet, intolerances, maxCarbs)
	if err != nil {
		return nil, fmt.Errorf("spoonacular search failed: %w", err)
	}

	foods := mapComplexResultsToFoods(recipes)
	_ = u.repo.UpsertFoods(ctx, foods) // Best-effort cache persist
	return foods, nil
}

// SearchByNutrients checks local DB with strict numeric bounds before calling API.
func (u *nutritionUseCase) SearchByNutrients(ctx context.Context, minProtein, maxFat, minCalories, maxCalories float64) ([]domain.Food, error) {
	localResults, err := u.repo.SearchFoodsByNutrients(ctx, minProtein, maxFat, minCalories, maxCalories)
	if err == nil && len(localResults) > 0 {
		return localResults, nil
	}

	if u.spoonClient == nil {
		return []domain.Food{}, nil
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
	if u.spoonClient == nil {
		return nil, fmt.Errorf("spoonacular client is not configured")
	}

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
		Source:          "custom",
		IsVerified:      true,
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

	suggestions, err := u.repo.GetRandomFoods(ctx, 5)
	if err != nil {
		suggestions = []domain.Food{}
	}

	burnedKcal, _ := u.workoutRepo.GetDailyBurnedCalories(ctx, userID, today)

	return &domain.DailyPlanResponse{
		TargetCalories:   2000.0,
		ConsumedCalories: consumedCalories,
		BurnedCalories:   burnedKcal,
		LoggedMeals:      logs,
		RecommendedFoods: suggestions,
	}, nil
}

// GetWeeklyAnalytics returns exactly 7 DailyAnalytics entries for the last 7
// days (6 days ago → today). Days with no data are filled with zeros.
func (u *nutritionUseCase) GetWeeklyAnalytics(ctx context.Context, userID uuid.UUID) (*domain.WeeklyAnalyticsResponse, error) {
	const numDays = 7

	// Fetch both maps concurrently for minimal latency
	type result struct {
		data map[string]float64
		err  error
	}
	cCh := make(chan result, 1)
	bCh := make(chan result, 1)

	go func() {
		d, err := u.repo.GetWeeklyConsumed(ctx, userID, numDays)
		cCh <- result{d, err}
	}()
	go func() {
		d, err := u.repo.GetWeeklyBurned(ctx, userID, numDays)
		bCh <- result{d, err}
	}()

	cRes := <-cCh
	bRes := <-bCh

	if cRes.err != nil {
		return nil, cRes.err
	}
	if bRes.err != nil {
		return nil, bRes.err
	}

	consumed := cRes.data
	if consumed == nil {
		consumed = map[string]float64{}
	}
	burned := bRes.data
	if burned == nil {
		burned = map[string]float64{}
	}

	// Build the exactly-7-entry slice from 6 days ago → today
	days := make([]domain.DailyAnalytics, numDays)
	today := time.Now()
	for i := 0; i < numDays; i++ {
		d := today.AddDate(0, 0, -(numDays-1-i))
		key := d.Format("2006-01-02")
		days[i] = domain.DailyAnalytics{
			Date:     key,
			Consumed: consumed[key], // 0 if absent
			Burned:   burned[key],   // 0 if absent
		}
	}

	return &domain.WeeklyAnalyticsResponse{Days: days}, nil
}


// ---------------------------------------------------------------------------
// Mapping helpers — translate Spoonacular DTOs → domain.Food entities
// ---------------------------------------------------------------------------

func mapComplexResultsToFoods(recipes []spoonacular.RecipeResult) []domain.Food {
	foods := make([]domain.Food, 0, len(recipes))
	for _, r := range recipes {
		id := r.ID
		foods = append(foods, domain.Food{
			SpoonacularID:   &id,
			Code:            fmt.Sprintf("SPOON-%d", r.ID),
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
func (u *nutritionUseCase) UpdateFoodLog(ctx context.Context, userID, logID uuid.UUID, quantity float64) (*domain.MealLog, error) {
	// 1. Strict Validation
	if quantity <= 0 || quantity > 5000 {
		return nil, domain.ErrInvalidQuantity
	}

	var updatedLog *domain.MealLog

	// 2. Concurrency Control (Transaction)
	err := u.repo.WithTransaction(ctx, func(txRepo domain.NutritionRepository) error {
		// 3. Pessimistic Locking + Ownership check (Anti-Enumeration via ErrLogNotFound)
		log, err := txRepo.GetMealLogForUpdate(ctx, logID, userID)
		if err != nil {
			return err
		}

		// 4. Recalculate Snapshot Macros
		// Assuming base nutrition is per 100g
		ratio := quantity / 100.0
		log.QuantityGrams = quantity
		log.CaloriesConsumed = log.Food.CaloriesPer100g * ratio
		log.ProteinConsumed = log.Food.ProteinPer100g * ratio
		log.FatConsumed = log.Food.FatPer100g * ratio
		log.CarbsConsumed = log.Food.CarbsPer100g * ratio

		// 5. Atomic Update
		if err := txRepo.UpdateMealLog(ctx, log); err != nil {
			return err
		}

		updatedLog = log
		return nil
	})

	if err != nil {
		return nil, err
	}

	return updatedLog, nil
}
