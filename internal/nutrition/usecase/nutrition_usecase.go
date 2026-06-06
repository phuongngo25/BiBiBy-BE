package usecase

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/service"
	"nutrix-backend/pkg/spoonacular"
)

type nutritionUseCase struct {
	repo                domain.NutritionRepository
	streakRepo          domain.StreakRepository
	achievementRepo      domain.AchievementRepository
	spoonClient         *spoonacular.Client
	workoutRepo         domain.WorkoutRepository
	userRepo            domain.UserRepository
	analyticsService    service.AnalyticsAggregationService
	streakService       service.StreakEvaluationService
	gamificationService service.GamificationService
	aiPort              domain.InferencePort
}

// NewNutritionUseCase creates the usecase wired with the nutrition repository, streak repository, Spoonacular client, workout repository and user repository.
func NewNutritionUseCase(
	repo domain.NutritionRepository,
	streakRepo domain.StreakRepository,
	achievementRepo domain.AchievementRepository,
	spoonClient *spoonacular.Client,
	workoutRepo domain.WorkoutRepository,
	userRepo domain.UserRepository,
	aiPort domain.InferencePort,
) domain.NutritionUseCase {
	analyticsSvc := service.NewAnalyticsAggregationService(repo, userRepo)
	var streakSvc service.StreakEvaluationService
	if streakRepo != nil {
		streakSvc = service.NewStreakEvaluationService(repo, streakRepo, userRepo, analyticsSvc)
	}

	var gamificationSvc service.GamificationService
	if achievementRepo != nil && streakRepo != nil {
		gamificationSvc = service.NewGamificationService(achievementRepo, streakRepo, analyticsSvc, userRepo)
	}

	return &nutritionUseCase{
		repo:                repo,
		streakRepo:          streakRepo,
		achievementRepo:      achievementRepo,
		spoonClient:         spoonClient,
		workoutRepo:         workoutRepo,
		userRepo:            userRepo,
		analyticsService:    analyticsSvc,
		streakService:       streakSvc,
		gamificationService: gamificationSvc,
		aiPort:              aiPort,
	}
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
	u.evaluateStreakHook(ctx, userID, date)
	return mealLog, nil
}

// GetDailyPlan builds a summary of a specific day's consumed macros and suggests new foods.
func (u *nutritionUseCase) GetDailyPlan(ctx context.Context, userID uuid.UUID, dateStr string) (*domain.DailyPlanResponse, error) {
	requestedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	log.Printf("[DAILY_PLAN] parsedDate=%v", requestedDate)

	logs, err := u.repo.GetDailyLogs(ctx, userID, requestedDate)
	if err != nil {
		return nil, err
	}

	var consumedCalories float64
	for _, l := range logs {
		consumedCalories += l.CaloriesConsumed
	}
	log.Printf("[DAILY_PLAN] mealsReturned=%d consumedCalories=%.1f", len(logs), consumedCalories)

	suggestions, err := u.repo.GetRandomFoods(ctx, 5)
	if err != nil {
		suggestions = []domain.Food{}
	}

	burnedKcal, _ := u.workoutRepo.GetDailyBurnedCalories(ctx, userID, requestedDate)

	targetCalories := 2000.0
	targetWater := 2000
	userProfile, err := u.userRepo.GetByID(ctx, userID)
	if err == nil && userProfile != nil {
		// Resolve snapshot date at midnight UTC (DATE column mapping)
		snapshotDate := time.Date(requestedDate.Year(), requestedDate.Month(), requestedDate.Day(), 0, 0, 0, 0, time.UTC)

		calcService := service.NewHealthCalculationService()
		bmr := int(math.Round(userProfile.BMR))
		if bmr <= 0 && userProfile.WeightKg > 0 && userProfile.HeightCm > 0 && userProfile.DOB != nil {
			bmr = calcService.CalculateBMR(userProfile.WeightKg, userProfile.HeightCm, *userProfile.DOB, userProfile.Gender)
		}
		tdee := int(math.Round(userProfile.TDEE))
		if tdee <= 0 && bmr > 0 {
			tdee = calcService.CalculateTDEE(bmr, userProfile.ActivityLevel)
		}

		strategy := service.NewGoalStrategyV1()
		targetCal, targetWat := strategy.CalculateTargets(userProfile)

		draft := &domain.DailyHealthSnapshot{
			UserID:              userID,
			SnapshotDate:        snapshotDate,
			WeightKg:            userProfile.WeightKg,
			ActivityLevel:       userProfile.ActivityLevel,
			GoalType:            userProfile.GoalType,
			BMR:                 bmr,
			TDEE:                tdee,
			TargetCalories:      targetCal,
			TargetWater:         targetWat,
			GoalStrategyVersion: "v1",
		}

		snapshot, snapErr := u.repo.GetOrCreateSnapshot(ctx, draft)
		if snapErr == nil && snapshot != nil {
			targetCalories = float64(snapshot.TargetCalories)
			targetWater = snapshot.TargetWater
			u.evaluateStreakHook(ctx, userID, snapshotDate)
		} else {
			// Fallback in case snapshot creation fails
			targetCalories = float64(targetCal)
			targetWater = targetWat
		}
	}

	consumedWater := 0
	cw, err := u.repo.GetDailyConsumedWater(ctx, userID, requestedDate)
	if err == nil {
		consumedWater = cw
	}

	return &domain.DailyPlanResponse{
		TargetCalories:   targetCalories,
		ConsumedCalories: consumedCalories,
		BurnedCalories:   burnedKcal,
		TargetWater:      targetWater,
		ConsumedWater:    consumedWater,
		LoggedMeals:      logs,
		RecommendedFoods: suggestions,
	}, nil
}


// GetDayAnalytics returns analytics for a single calendar day.
func (u *nutritionUseCase) GetDayAnalytics(ctx context.Context, userID uuid.UUID, dateStr string) (*domain.DayAnalytics, error) {
	requestedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}

	days, err := u.analyticsService.BuildAnalyticsRange(ctx, userID, requestedDate, requestedDate)
	if err != nil {
		return nil, err
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("could not retrieve analytics for date")
	}

	return &days[0], nil
}

// GetWeeklyAnalytics returns week-over-week nutritional tracking, configurable totals, and metadata.
func (u *nutritionUseCase) GetWeeklyAnalytics(ctx context.Context, userID uuid.UUID, daysCount int, isCalendar bool) (*domain.WeeklyAnalyticsResponse, error) {
	if daysCount <= 0 {
		daysCount = 7
	}

	var start, end time.Time
	
	// Resolve timezone from user profile if available, to make rolling/calendar window calculations timezone-safe.
	loc := time.UTC
	userProfile, err := u.userRepo.GetByID(ctx, userID)
	if err == nil && userProfile != nil && userProfile.Timezone != "" {
		if l, errLoc := time.LoadLocation(userProfile.Timezone); errLoc == nil {
			loc = l
		}
	}
	today := time.Now().In(loc)

	if isCalendar {
		// Calculate current calendar week (Monday to Sunday) relative to UTC midnights
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday is 7th day
		}
		start = today.AddDate(0, 0, -(weekday - 1))
		end = start.AddDate(0, 0, 6)
	} else {
		// Rolling days (default: 6 days ago -> today)
		end = today
		start = today.AddDate(0, 0, -(daysCount - 1))
	}

	days, err := u.analyticsService.BuildAnalyticsRange(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	var totalConsumed, totalBurned, totalWater int
	for _, d := range days {
		totalConsumed += d.ConsumedCalories
		totalBurned += d.WorkoutBurned
		totalWater += d.ConsumedWater
	}

	// Calculate active streak within the week range (scanned backwards from the end of the range)
	streak := 0
	for i := len(days) - 1; i >= 0; i-- {
		if days[i].GoalHit {
			streak++
		} else {
			break
		}
	}

	weeklyType := "rolling"
	if isCalendar {
		weeklyType = "calendar"
	}

	return &domain.WeeklyAnalyticsResponse{
		Type:                   weeklyType,
		WindowDays:             len(days),
		StartDate:              start.Format("2006-01-02"),
		EndDate:                end.Format("2006-01-02"),
		Days:                   days,
		WeeklyConsumedCalories: totalConsumed,
		WeeklyBurnedCalories:   totalBurned,
		WeeklyConsumedWater:    totalWater,
		StreakDays:             streak,
	}, nil
}

// GetMonthlyAnalytics aggregates daily records and calculates streaks deterministically relative to the month's end.
func (u *nutritionUseCase) GetMonthlyAnalytics(ctx context.Context, userID uuid.UUID, monthStr string) (*domain.MonthlyAnalyticsResponse, error) {
	parsed, err := time.Parse("2006-01", monthStr)
	if err != nil {
		return nil, fmt.Errorf("invalid month format: %w", err)
	}

	start := time.Date(parsed.Year(), parsed.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)

	days, err := u.analyticsService.BuildAnalyticsRange(ctx, userID, start, end)
	if err != nil {
		return nil, err
	}

	var totalConsumed, totalWater int
	var goalHitDays, waterHitDays, calorieHitDays int

	for _, d := range days {
		totalConsumed += d.ConsumedCalories
		totalWater += d.ConsumedWater
		if d.GoalHit {
			goalHitDays++
		}
		if d.WaterGoalHit {
			waterHitDays++
		}
		if d.CalorieGoalHit {
			calorieHitDays++
		}
	}

	// Compute monthly consecutive runs (Longest vs Current)
	longestRun := 0
	currentRun := 0
	tempRun := 0

	for _, d := range days {
		if d.GoalHit {
			tempRun++
			if tempRun > longestRun {
				longestRun = tempRun
			}
		} else {
			tempRun = 0
		}
	}

	// Calculate current active run ending on the last day of the month (deterministic)
	for i := len(days) - 1; i >= 0; i-- {
		if days[i].GoalHit {
			currentRun++
		} else {
			break
		}
	}

	return &domain.MonthlyAnalyticsResponse{
		Days:                  days,
		TotalConsumedCalories: totalConsumed,
		TotalConsumedWater:    totalWater,
		GoalHitDays:           goalHitDays,
		WaterGoalHitDays:      waterHitDays,
		CalorieGoalHitDays:    calorieHitDays,
		LongestGoalHitRun:     longestRun,
		CurrentGoalHitRun:     currentRun,
	}, nil
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

	if updatedLog != nil {
		u.evaluateStreakHook(ctx, userID, updatedLog.ConsumedDate)
	}

	return updatedLog, nil
}

// GetJobStatus serves as a proxy to the Orchestrator's JobStore for the HTTP Delivery layer.
// In a full integration, the Orchestrator/JobStore would be injected into this UseCase.
func (u *nutritionUseCase) GetJobStatus(ctx context.Context, jobID string) (*domain.JobStatusResponse, error) {
	// MOCK INTEGRATION: Simulating JobStore retrieval to satisfy the strict API contract.
	if jobID == "" {
		return nil, fmt.Errorf("job not found")
	}

	return &domain.JobStatusResponse{
		ID:        jobID,
		Type:      "meal_plan",
		Status:    "failed",
		Done:      true,
		Error:     "panic: runtime error: invalid memory address or nil pointer dereference",
		UpdatedAt: time.Now(),
	}, nil
}

// LogWater records a hydration event for the user.
func (u *nutritionUseCase) LogWater(ctx context.Context, userID uuid.UUID, req *domain.LogWaterRequest) (*domain.LogWaterResponse, error) {
	if req.AmountMl <= 0 || req.AmountMl > 2000 {
		return nil, domain.ErrInvalidQuantity
	}
	source := strings.TrimSpace(req.Source)
	if len(source) > 50 {
		source = source[:50]
	}

	log := &domain.WaterLog{
		UserID:   userID,
		AmountMl: req.AmountMl,
		Source:   source,
	}

	if err := u.repo.LogWater(ctx, log); err != nil {
		return nil, domain.ErrInternalServerError
	}

	u.evaluateStreakHook(ctx, userID, time.Now())

	return &domain.LogWaterResponse{
		ID:        log.ID,
		AmountMl:  log.AmountMl,
		Source:    log.Source,
		CreatedAt: log.CreatedAt,
	}, nil
}

// GetStreak retrieves the cached streak for the user. If none exists, it lazy-evaluates and caches it on the fly.
func (u *nutritionUseCase) GetStreak(ctx context.Context, userID uuid.UUID) (*domain.UserStreak, error) {
	if u.streakRepo == nil {
		return &domain.UserStreak{UserID: userID}, nil
	}

	streak, err := u.streakRepo.GetStreak(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Cold-start self-healing: lazy evaluate if no cached record exists
	if streak == nil {
		if u.streakService != nil {
			loc := time.UTC
			userProfile, err := u.userRepo.GetByID(ctx, userID)
			if err == nil && userProfile != nil && userProfile.Timezone != "" {
				if l, errLoc := time.LoadLocation(userProfile.Timezone); errLoc == nil {
					loc = l
				}
			}
			today := time.Now().In(loc)
			
			streak, err = u.streakService.EvaluateStreak(ctx, userID, today)
			if err != nil {
				log.Printf("[GetStreak] dynamic evaluation failed for user %s: %v", userID, err)
				return &domain.UserStreak{UserID: userID}, nil
			}
		} else {
			return &domain.UserStreak{UserID: userID}, nil
		}
	}

	return streak, nil
}

// evaluateStreakHook triggers the StreakEvaluationService timezone-safely in a background goroutine.
// As part of the resilient failure policy, cache failures never rollback log mutations.
func (u *nutritionUseCase) evaluateStreakHook(ctx context.Context, userID uuid.UUID, date time.Time) {
	if u.streakService == nil {
		return
	}
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := u.streakService.EvaluateStreak(bgCtx, userID, date)
		if err != nil {
			log.Printf("[Streak Hook] WARNING: Streak evaluation failed for user %s on date %v: %v", userID, date, err)
		} else {
			log.Printf("[Streak Hook] Success: Streak evaluated for user %s on date %v", userID, date)

			// Trigger achievement evaluation after successful streak update
			if u.gamificationService != nil {
				// Evaluate streak-based achievements
				if err := u.gamificationService.EvaluateAchievements(bgCtx, userID, domain.TriggerStreakUpdated, date); err != nil {
					log.Printf("[Achievement Hook] WARNING: Streak achievement evaluation failed for user %s: %v", userID, err)
				}
				// Evaluate goal-based achievements for this specific date
				if err := u.gamificationService.EvaluateAchievements(bgCtx, userID, domain.TriggerDailyGoal, date); err != nil {
					log.Printf("[Achievement Hook] WARNING: Daily goal achievement evaluation failed for user %s on date %v: %v", userID, date, err)
				}
			}
		}
	}()
}
var AIFoodRegistry = map[string]string{
	"banh_beo": "de9a1cbd-27ed-4cf6-84c4-ac23ae49c7ce",
	"banh_bot_loc": "8c503823-19d0-4248-9f33-f9320e9c4e7d",
	"banh_can": "3b607f03-496c-4082-abc1-b1836df08638",
	"banh_canh": "cbe50f45-9654-4799-ab1f-97ec683477c3",
	"banh_chung": "09982e82-ce84-440d-9c5c-ba70ed20ae38",
	"banh_cuon": "017ec832-5da8-4a70-9c71-a78263a88b31",
	"banh_duc": "3f32e1da-c56a-4236-8c5c-0cb4c9e5e178",
	"banh_gio": "11f2d505-1cc1-4e03-b7cb-028c88aa36a1",
	"banh_khot": "73b1f6da-170a-4554-9d13-42046c0e14e8",
	"banh_mi": "a3b7fdae-f6cd-49e9-82f3-f51ec3a534ce",
	"banh_pia": "e2c3a6e5-aa49-45c7-8b22-bb0dc6349fa2",
	"banh_tet": "a5a366be-324c-4b40-98e5-df1a4b7e84fc",
	"banh_trang_nuong": "a3b78b44-3191-4d47-9bde-c5b3de226982",
	"banh_xeo": "df43cbf0-2f5b-4f80-b575-0d28f15c3a49",
	"bun_bo_hue": "d34e809b-c634-41ab-8bf2-7086506be757",
	"bun_dau_mam_tom": "c61c7e67-dc2d-4f97-a252-a4e04db2f88a",
	"bun_mam": "64479a47-d57a-4b19-b2e2-6d7bbe00dbf9",
	"bun_rieu": "5c543961-42b2-43c4-a821-0cdf74d7691c",
	"bun_thit_nuong": "70625c41-0f01-4544-9ad3-3f2ef8a27881",
	"ca_kho_to": "e6c966f0-f6d0-4328-8c5f-fd7c6be44741",
	"canh_chua": "d54d6238-e299-4074-b980-54aa66dcdda0",
	"cao_lau": "8a178f5e-d889-4ea0-a727-4824c8f9e80d",
	"chao_long": "7788269d-2955-428a-9d61-efa6181ca44f",
	"com_tam": "db17b81e-e5e2-4a49-ae75-022b58169c20",
	"goi_cuon": "4674dd55-c4d0-43b8-ba94-949abee2e5f2",
	"hu_tieu": "455231e5-daf8-4a66-a9f3-0fcd305113d7",
	"mi_quang": "d2e673de-ae0e-4546-b390-f6cfdf85d467",
	"nem_chua": "42fd673e-11b5-4298-9353-85cb7e4ef371",
	"pho": "525514d7-4261-4d7d-8cd6-6c5bcf28646f",
	"xoi_xeo": "25163e9b-a507-4e51-ba5d-e66cdfcf35d0",
}

func (u *nutritionUseCase) EstimateNutrition(ctx context.Context, imageBytes []byte) (*domain.FoodEstimateResponse, error) {
	result, err := u.aiPort.EstimateVolume(ctx, imageBytes)
	if err != nil {
		return nil, fmt.Errorf("could not estimate from image: %w", err)
	}

	if result.FoodLabel == "unknown" || result.FoodLabel == "" {
		return nil, fmt.Errorf("ai could not recognize food in image")
	}

	dishIDStr, exists := AIFoodRegistry[result.FoodLabel]
	if !exists {
		return nil, fmt.Errorf("unsupported food label from AI: %s", result.FoodLabel)
	}

	parsedDishID, err := uuid.Parse(dishIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid dish ID format in registry: %w", err)
	}

	foodInfo, err := u.repo.GetFoodByID(ctx, parsedDishID)
	if err != nil {
		return nil, fmt.Errorf("food item not found in DB: %w", err)
	}

	massGrams := result.MassG
	ratio := massGrams / 100.0

	return &domain.FoodEstimateResponse{
		FoodID:          foodInfo.ID,
		FoodLabel:       result.FoodLabel,
		Name:            foodInfo.Name,
		Calories:        foodInfo.CaloriesPer100g * ratio,
		Protein:         foodInfo.ProteinPer100g * ratio,
		Fat:             foodInfo.FatPer100g * ratio,
		Carbs:           foodInfo.CarbsPer100g * ratio,
		CaloriesPer100g: foodInfo.CaloriesPer100g,
		ProteinPer100g:  foodInfo.ProteinPer100g,
		FatPer100g:      foodInfo.FatPer100g,
		CarbsPer100g:    foodInfo.CarbsPer100g,
		QuantityGrams:   massGrams,
		Confidence:      result.FoodLabelConfidence,
	}, nil
}
