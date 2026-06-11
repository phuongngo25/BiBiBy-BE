package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/infrastructure/metrics"
	"nutrix-backend/internal/nutrition/service"
	"nutrix-backend/pkg/spoonacular"
)

type nutritionUseCase struct {
	repo                domain.NutritionRepository
	streakRepo          domain.StreakRepository
	achievementRepo     domain.AchievementRepository
	spoonClient         *spoonacular.Client
	workoutRepo         domain.WorkoutRepository
	userRepo            domain.UserRepository
	portfolioRepo       domain.UserPortfolioRepository
	analyticsService    service.AnalyticsAggregationService
	streakService       service.StreakEvaluationService
	gamificationService service.GamificationService
	cvPort              domain.InferencePort
	kgPort              domain.NutritionIntelligencePort
	redis               *redis.Client

	planCache map[string]*domain.WeeklyPlanResponseDTO
	mu        sync.RWMutex
}

// NewNutritionUseCase creates the usecase wired with dependencies.
func NewNutritionUseCase(
	repo domain.NutritionRepository,
	streakRepo domain.StreakRepository,
	achievementRepo domain.AchievementRepository,
	spoonClient *spoonacular.Client,
	workoutRepo domain.WorkoutRepository,
	userRepo domain.UserRepository,
	cvPort domain.InferencePort,
	kgPort domain.NutritionIntelligencePort,
	redis *redis.Client,
	portfolioRepos ...domain.UserPortfolioRepository,
) domain.NutritionUseCase {
	var portfolioRepo domain.UserPortfolioRepository
	if len(portfolioRepos) > 0 {
		portfolioRepo = portfolioRepos[0]
	}
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
		achievementRepo:     achievementRepo,
		spoonClient:         spoonClient,
		workoutRepo:         workoutRepo,
		userRepo:            userRepo,
		portfolioRepo:       portfolioRepo,
		analyticsService:    analyticsSvc,
		streakService:       streakSvc,
		gamificationService: gamificationSvc,
		cvPort:              cvPort,
		kgPort:              kgPort,
		redis:               redis,
		planCache:           make(map[string]*domain.WeeklyPlanResponseDTO),
	}
}

// SearchFoods is enriched with KG Metadata (Sprint 1A).
func (u *nutritionUseCase) SearchFoods(ctx context.Context, keyword string) ([]domain.Food, error) {
	if strings.TrimSpace(keyword) == "" {
		return []domain.Food{}, nil
	}

	// 1. Local Search
	localResults, _ := u.repo.SearchFoods(ctx, keyword)

	var rawResults []domain.Food
	if len(localResults) > 0 {
		rawResults = localResults
	} else if u.spoonClient != nil {
		recipes, spErr := u.spoonClient.ComplexSearch(ctx, keyword, "", "", 0)
		if spErr == nil && len(recipes) > 0 {
			rawResults = mapComplexResultsToFoods(recipes)
			go func(foods []domain.Food) {
				_ = u.repo.UpsertFoods(context.Background(), foods)
			}(rawResults)
		}
	}

	if len(rawResults) == 0 {
		return []domain.Food{}, nil
	}

	// 2. KG Batch Enrichment with Redis Caching (Sprint 1AA)
	diseaseIDs := []string{}
	profileHash := "default_profile"

	missingFoodIDs := make([]string, 0, len(rawResults))
	enrichedMap := make(map[string]domain.BatchFoodMetadata)

	for _, f := range rawResults {
		foodID := f.ID.String()
		cacheKey := fmt.Sprintf("kg:v2:food:%s:risk:%s", foodID, profileHash)

		if u.redis != nil {
			cached, err := u.redis.Get(ctx, cacheKey).Result()
			if err == nil && cached != "" {
				var meta domain.KGMetadata
				if json.Unmarshal([]byte(cached), &meta) == nil {
					enrichedMap[foodID] = domain.BatchFoodMetadata{KGMetadata: &meta}
					continue
				}
			}
		}
		missingFoodIDs = append(missingFoodIDs, foodID)
	}

	if len(missingFoodIDs) > 0 {
		if u.kgPort != nil {
			log.Printf("[SearchFoods] KG Cache MISS for %d foods -> calling AI Server", len(missingFoodIDs))
			newEnriched, err := u.kgPort.BatchAnalyzeFoods(ctx, missingFoodIDs, diseaseIDs)
			if err == nil {
				for fID, metadata := range newEnriched {
					enrichedMap[fID] = metadata
					if u.redis != nil && metadata.KGMetadata != nil {
						cacheKey := fmt.Sprintf("kg:v2:food:%s:risk:%s", fID, profileHash)
						jsonData, _ := json.Marshal(metadata.KGMetadata)
						u.redis.Set(ctx, cacheKey, string(jsonData), 24*time.Hour)
					}
				}
			} else {
				log.Printf("[SearchFoods] KG Enrichment failed: %v", err)
			}
		}
	} else {
		log.Printf("[SearchFoods] KG Cache HIT for all %d foods", len(rawResults))
	}

	for i := range rawResults {
		if metadata, ok := enrichedMap[rawResults[i].ID.String()]; ok {
			rawResults[i].KGMetadata = metadata.KGMetadata
		} else {
			rawResults[i].KGMetadata = &domain.KGMetadata{
				IsSafe:    false,
				RiskLevel: "UNKNOWN",
				Warnings: []domain.KGWarning{
					{Code: "SRE_UNAVAILABLE", Message: "Safety analysis unavailable", Severity: "RISK_UNSPECIFIED"},
				},
			}
		}
	}

	return rawResults, nil
}

func (u *nutritionUseCase) SearchSpoonacular(ctx context.Context, query, diet, intolerances string, maxCarbs int) ([]domain.Food, error) {
	if u.spoonClient == nil {
		return nil, fmt.Errorf("spoonacular client is not configured")
	}
	if diet == "" && intolerances == "" {
		localResults, err := u.repo.SearchFoods(ctx, query)
		if err == nil && len(localResults) > 0 {
			return localResults, nil
		}
	}
	recipes, err := u.spoonClient.ComplexSearch(ctx, query, diet, intolerances, maxCarbs)
	if err != nil {
		return nil, fmt.Errorf("spoonacular search failed: %w", err)
	}
	foods := mapComplexResultsToFoods(recipes)
	_ = u.repo.UpsertFoods(ctx, foods)
	return foods, nil
}

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

func (u *nutritionUseCase) GetDailyPlan(ctx context.Context, userID uuid.UUID, dateStr string) (*domain.DailyPlanResponse, error) {
	requestedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}
	logs, err := u.repo.GetDailyLogs(ctx, userID, requestedDate)
	if err != nil {
		return nil, err
	}
	var consumedCalories float64
	for _, l := range logs {
		consumedCalories += l.CaloriesConsumed
	}
	suggestions, err := u.repo.GetRandomFoods(ctx, 5)
	if err != nil {
		suggestions = []domain.Food{}
	}
	burnedKcal, _ := u.workoutRepo.GetDailyBurnedCalories(ctx, userID, requestedDate)
	targetCalories := 2000.0
	targetWater := 2000
	waterDate := requestedDate
	userProfile, err := u.userRepo.GetByID(ctx, userID)
	if err == nil && userProfile != nil {
		loc := time.UTC
		if userProfile.Timezone != "" {
			if l, errLoc := time.LoadLocation(userProfile.Timezone); errLoc == nil {
				loc = l
			}
		}
		waterDate = time.Date(requestedDate.Year(), requestedDate.Month(), requestedDate.Day(), 0, 0, 0, 0, loc)
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
			targetCalories = float64(targetCal)
			targetWater = targetWat
		}
		if portfolio := u.loadPlannerPortfolio(ctx, userID); portfolio != nil {
			if portfolio.DailyWaterTargetML > 0 {
				targetWater = portfolio.DailyWaterTargetML
			}
			if portfolio.CalorieTargetOverride != nil && *portfolio.CalorieTargetOverride > 0 {
				targetCalories = *portfolio.CalorieTargetOverride
			}
		}
	}
	consumedWater := 0
	cw, err := u.repo.GetDailyConsumedWater(ctx, userID, waterDate)
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

func (u *nutritionUseCase) GetWeeklyAnalytics(ctx context.Context, userID uuid.UUID, daysCount int, isCalendar bool) (*domain.WeeklyAnalyticsResponse, error) {
	if daysCount <= 0 {
		daysCount = 7
	}
	var start, end time.Time
	loc := time.UTC
	userProfile, err := u.userRepo.GetByID(ctx, userID)
	if err == nil && userProfile != nil && userProfile.Timezone != "" {
		if l, errLoc := time.LoadLocation(userProfile.Timezone); errLoc == nil {
			loc = l
		}
	}
	today := time.Now().In(loc)
	if isCalendar {
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = today.AddDate(0, 0, -(weekday - 1))
		end = start.AddDate(0, 0, 6)
	} else {
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
		LongestGoalHitRun:     longestRun,
		CurrentGoalHitRun:     currentRun,
	}, nil
}

func (u *nutritionUseCase) GetRecommendations(ctx context.Context, userID uuid.UUID, req *domain.GetRecommendationsRequest) (*domain.GetRecommendationsResponse, error) {
	if u.kgPort == nil {
		return nil, fmt.Errorf("KG service unavailable")
	}

	return u.kgPort.GetRecommendations(ctx, userID, req)
}

func (u *nutritionUseCase) ExplainFood(ctx context.Context, userID uuid.UUID, foodID string) (*domain.FoodExplanation, error) {
	if u.kgPort == nil {
		return nil, fmt.Errorf("KG service unavailable")
	}

	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user context: %w", err)
	}

	diseaseStr := strings.TrimSpace(user.MedicalConditions)
	var diseases []domain.UserDisease
	if diseaseStr != "" {
		parts := strings.Split(diseaseStr, ",")
		for _, part := range parts {
			diseases = append(diseases, domain.UserDisease{ID: strings.TrimSpace(part)})
		}
	}

	userCtx := domain.UserNutritionContext{
		Diseases: diseases,
	}

	return u.kgPort.ExplainFood(ctx, userCtx, foodID)
}

func (u *nutritionUseCase) UpdateFoodLog(ctx context.Context, userID, logID uuid.UUID, quantity float64) (*domain.MealLog, error) {
	if quantity <= 0 || quantity > 5000 {
		return nil, domain.ErrInvalidQuantity
	}
	var updatedLog *domain.MealLog
	err := u.repo.WithTransaction(ctx, func(txRepo domain.NutritionRepository) error {
		log, err := txRepo.GetMealLogForUpdate(ctx, logID, userID)
		if err != nil {
			return err
		}
		ratio := quantity / 100.0
		log.QuantityGrams = quantity
		log.CaloriesConsumed = log.Food.CaloriesPer100g * ratio
		log.ProteinConsumed = log.Food.ProteinPer100g * ratio
		log.FatConsumed = log.Food.FatPer100g * ratio
		log.CarbsConsumed = log.Food.CarbsPer100g * ratio
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

func (u *nutritionUseCase) GetJobStatus(ctx context.Context, jobID string) (*domain.JobStatusResponse, error) {
	if jobID == "" {
		return nil, fmt.Errorf("job not found")
	}
	return &domain.JobStatusResponse{
		ID: jobID, Type: "meal_plan", Status: "failed", Done: true,
		Error:     "panic: runtime error: invalid memory address or nil pointer dereference",
		UpdatedAt: time.Now(),
	}, nil
}

func (u *nutritionUseCase) LogWater(ctx context.Context, userID uuid.UUID, req *domain.LogWaterRequest) (*domain.LogWaterResponse, error) {
	if req.AmountMl <= 0 || req.AmountMl > 2000 {
		return nil, domain.ErrInvalidQuantity
	}
	source := strings.TrimSpace(req.Source)
	if len(source) > 50 {
		source = source[:50]
	}
	log := &domain.WaterLog{UserID: userID, AmountMl: req.AmountMl, Source: source}
	if err := u.repo.LogWater(ctx, log); err != nil {
		return nil, domain.ErrInternalServerError
	}
	u.evaluateStreakHook(ctx, userID, time.Now())
	return &domain.LogWaterResponse{ID: log.ID, AmountMl: log.AmountMl, Source: log.Source, CreatedAt: log.CreatedAt}, nil
}

func (u *nutritionUseCase) GetStreak(ctx context.Context, userID uuid.UUID) (*domain.UserStreak, error) {
	if u.streakRepo == nil {
		return &domain.UserStreak{UserID: userID}, nil
	}
	streak, err := u.streakRepo.GetStreak(ctx, userID)
	if err != nil {
		return nil, err
	}
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
				return &domain.UserStreak{UserID: userID}, nil
			}
		} else {
			return &domain.UserStreak{UserID: userID}, nil
		}
	}
	return streak, nil
}

func (u *nutritionUseCase) evaluateStreakHook(ctx context.Context, userID uuid.UUID, date time.Time) {
	if u.streakService == nil {
		return
	}
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := u.streakService.EvaluateStreak(bgCtx, userID, date)
		if err == nil && u.gamificationService != nil {
			_ = u.gamificationService.EvaluateAchievements(bgCtx, userID, domain.TriggerStreakUpdated, date)
			_ = u.gamificationService.EvaluateAchievements(bgCtx, userID, domain.TriggerDailyGoal, date)
		}
	}()
}

func (u *nutritionUseCase) EstimateNutrition(ctx context.Context, imageBytes []byte) (*domain.FoodEstimateResponse, error) {
	if u.cvPort == nil {
		return nil, fmt.Errorf("CV service unavailable")
	}
	result, err := u.cvPort.EstimateVolume(ctx, imageBytes)
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
	parsedDishID, _ := uuid.Parse(dishIDStr)
	foodInfo, err := u.repo.GetFoodByID(ctx, parsedDishID)
	if err != nil {
		return nil, fmt.Errorf("food item not found in DB: %w", err)
	}
	ratio := result.MassG / 100.0
	return &domain.FoodEstimateResponse{
		FoodID: foodInfo.ID, FoodLabel: result.FoodLabel, Name: foodInfo.Name,
		Calories: foodInfo.CaloriesPer100g * ratio, Protein: foodInfo.ProteinPer100g * ratio,
		Fat: foodInfo.FatPer100g * ratio, Carbs: foodInfo.CarbsPer100g * ratio,
		CaloriesPer100g: foodInfo.CaloriesPer100g, ProteinPer100g: foodInfo.ProteinPer100g,
		FatPer100g: foodInfo.FatPer100g, CarbsPer100g: foodInfo.CarbsPer100g,
		QuantityGrams: result.MassG, Confidence: result.FoodLabelConfidence,
	}, nil
}

func (u *nutritionUseCase) GetThresholdSnapshot(ctx context.Context, userID uuid.UUID) (*domain.ThresholdSnapshot, error) {
	if u.kgPort == nil {
		return nil, fmt.Errorf("KG service unavailable")
	}

	user, err := u.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user context: %w", err)
	}

	var diseaseIDs []string
	if user.MedicalConditions != "" {
		for _, part := range strings.Split(user.MedicalConditions, ",") {
			diseaseIDs = append(diseaseIDs, strings.TrimSpace(part))
		}
	}

	return u.kgPort.GetThresholdSnapshot(ctx, diseaseIDs)
}

func (u *nutritionUseCase) SubmitFoodFeedback(ctx context.Context, userID uuid.UUID, req *domain.FoodFeedbackRequest) error {
	if u.kgPort == nil {
		return fmt.Errorf("KG service unavailable")
	}

	requestID := firstNonEmpty(
		req.RequestID,
		stringFromMetadata(req.Metadata, "request_id"),
		fmt.Sprintf("req-%d", time.Now().UnixNano()),
	)
	predictedFoodID := firstNonEmpty(
		req.PredictedFoodID,
		req.FoodID,
		stringFromMetadata(req.Metadata, "predicted_food_id"),
	)
	predictedFoodName := firstNonEmpty(
		req.PredictedFoodName,
		stringFromMetadata(req.Metadata, "predicted_food_name"),
		"Unknown",
	)
	finalFoodID := firstNonEmpty(
		req.FinalFoodID,
		stringFromMetadata(req.Metadata, "final_food_id"),
		req.FoodID,
		predictedFoodID,
	)
	finalFoodName := firstNonEmpty(
		req.FinalFoodName,
		stringFromMetadata(req.Metadata, "final_food_name"),
		predictedFoodName,
	)
	confidence := firstPositiveFloat(
		req.PredictionConfidence,
		floatFromMetadata(req.Metadata, "prediction_confidence"),
		1.0,
	)
	imageHash := firstNonEmpty(req.ImageHash, stringFromMetadata(req.Metadata, "image_hash"))
	createdAt := firstPositiveInt64(
		req.CreatedAt,
		int64FromMetadata(req.Metadata, "created_at"),
		time.Now().UnixMilli(),
	)

	var err error
	switch req.UserAction {
	case "CORRECTION":
		err = u.kgPort.SubmitFoodCorrection(ctx, &domain.SubmitFoodCorrectionRequest{
			RequestID:            requestID,
			PredictedFoodID:      predictedFoodID,
			PredictedFoodName:    predictedFoodName,
			FinalFoodID:          finalFoodID,
			FinalFoodName:        finalFoodName,
			PredictionConfidence: confidence,
			ImageHash:            imageHash,
			CreatedAt:            createdAt,
		})
	case "ACCEPTANCE":
		err = u.kgPort.SubmitFoodAcceptance(ctx, &domain.SubmitFoodAcceptanceRequest{
			RequestID:            requestID,
			PredictedFoodID:      predictedFoodID,
			PredictedFoodName:    predictedFoodName,
			PredictionConfidence: confidence,
			ImageHash:            imageHash,
			CreatedAt:            createdAt,
		})
	case "VIEWED":
		err = u.kgPort.SubmitFoodViewed(ctx, &domain.SubmitFoodViewedRequest{
			RequestID:            requestID,
			PredictedFoodID:      predictedFoodID,
			PredictedFoodName:    predictedFoodName,
			PredictionConfidence: confidence,
			ImageHash:            imageHash,
			CreatedAt:            createdAt,
		})
	default:
		return fmt.Errorf("invalid user action: %s", req.UserAction)
	}

	if err == nil {
		metrics.FeedbackEventsTotal.WithLabelValues(req.UserAction).Inc()
	}

	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func stringFromMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}

	raw, ok := metadata[key]
	if !ok || raw == nil {
		return ""
	}

	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func floatFromMetadata(metadata map[string]any, key string) float64 {
	if metadata == nil {
		return 0
	}

	raw, ok := metadata[key]
	if !ok || raw == nil {
		return 0
	}

	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int32:
		return float64(value)
	case int64:
		return float64(value)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return parsed
		}
	}

	return 0
}

func int64FromMetadata(metadata map[string]any, key string) int64 {
	if metadata == nil {
		return 0
	}

	raw, ok := metadata[key]
	if !ok || raw == nil {
		return 0
	}

	switch value := raw.(type) {
	case int64:
		return value
	case int32:
		return int64(value)
	case int:
		return int64(value)
	case float64:
		return int64(value)
	case float32:
		return int64(value)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil {
			return parsed
		}
	}

	return 0
}

// ─── Planner / Meal Validation (Epic 1 Chunk C) ──────────────────

func (u *nutritionUseCase) GenerateWeeklyPlan(ctx context.Context, userID uuid.UUID, req *domain.GenerateWeeklyPlanRequest) (*domain.GenerateWeeklyPlanResponse, error) {
	startTime := time.Now()
	var statusCode string = "200"
	defer func() {
		metrics.PlannerGenerationDurationMs.WithLabelValues("GenerateWeeklyPlan", statusCode).Observe(float64(time.Since(startTime).Milliseconds()))
	}()

	candidateFoods := req.CandidateFoods
	generatedFromDB := false
	dbCandidates := map[string]domain.Food{}
	userPortfolio := u.loadPlannerPortfolio(ctx, userID)
	if len(candidateFoods) == 0 {
		foods, err := u.repo.GetRandomFoods(ctx, 100)
		if err != nil {
			statusCode = "500"
			return nil, fmt.Errorf("failed to load planner candidates: %w", err)
		}
		userProfile, _ := u.userRepo.GetByID(ctx, userID)
		foods = sortPlannerFoodsByPortfolio(foods, userPortfolio)
		candidateFoods = make([]string, 0, len(foods))
		for _, food := range foods {
			if !plannerFoodAllowedByProfile(food, userProfile, userPortfolio) {
				continue
			}
			foodID := food.ID.String()
			candidateFoods = append(candidateFoods, foodID)
			dbCandidates[foodID] = food
		}
		generatedFromDB = true
	}

	var validMeals []domain.PlannedMealDTO

	for i, candidateFoodID := range candidateFoods {
		mealID := fmt.Sprintf("meal-%d", i)
		candidate := domain.CandidateMeal{
			MealID:   mealID,
			FoodIDs:  []string{candidateFoodID},
			MealType: "lunch",
		}
		if food, ok := dbCandidates[candidateFoodID]; ok {
			candidate.Ingredients = plannerTerms(food.Name, food.NameEn, food.NameVi)
			candidate.Categories = plannerTerms(food.Category, food.Source)
			candidate.ProteinSources = plannerTerms(food.Name, food.NameEn, food.Category)
		}

		analyzeReq := &domain.AnalyzeMealRequest{
			Candidate: candidate,
		}

		analyzeResp, err := u.kgPort.AnalyzeMeal(ctx, analyzeReq)
		if err != nil {
			statusCode = "500"
			metrics.PlannerGenerationFailuresTotal.WithLabelValues("kg_error").Inc()
			return nil, fmt.Errorf("failed to analyze candidate meal: %w", err)
		}

		// Fail closed for hard REJECTED meals. For DB-generated candidates, KG
		// UNKNOWN/unavailable is treated as a coverage gap after local filtering.
		if analyzeResp.Status == "APPROVED" || analyzeResp.Status == "WARNING" || (generatedFromDB && plannerKGUnavailable(analyzeResp)) {
			status := analyzeResp.Status
			if status == "REJECTED" {
				status = "WARNING"
			}
			validMeals = append(validMeals, domain.PlannedMealDTO{
				MealID:   mealID,
				FoodIDs:  []string{candidateFoodID},
				MealType: "lunch",
				Status:   status,
			})
			if food, ok := dbCandidates[candidateFoodID]; ok {
				validMeals[len(validMeals)-1].FoodName = nonEmptyPlanner(food.Name, food.NameEn, food.NameVi, candidateFoodID)
				validMeals[len(validMeals)-1].Calories = food.CaloriesPer100g
				validMeals[len(validMeals)-1].Protein = food.ProteinPer100g
				validMeals[len(validMeals)-1].Carbs = food.CarbsPer100g
				validMeals[len(validMeals)-1].Fat = food.FatPer100g
			}
		}
	}

	if len(validMeals) == 0 {
		statusCode = "422"
		metrics.PlannerGenerationFailuresTotal.WithLabelValues("all_rejected").Inc()
		return nil, domain.ErrAllCandidatesRejected
	}

	meals := validMeals
	if generatedFromDB {
		meals = expandWeeklyPlanMeals(validMeals, req.StartDate, userPortfolio)
	}

	resp := &domain.GenerateWeeklyPlanResponse{
		WeeklyPlanResponseDTO: domain.WeeklyPlanResponseDTO{
			PlanID: uuid.New().String(),
			Meals:  meals,
		},
	}

	u.mu.Lock()
	u.planCache[resp.PlanID] = &resp.WeeklyPlanResponseDTO
	u.mu.Unlock()

	return resp, nil
}

func (u *nutritionUseCase) loadPlannerPortfolio(ctx context.Context, userID uuid.UUID) *domain.UserPortfolio {
	if u.portfolioRepo == nil {
		return nil
	}
	portfolio, err := u.portfolioRepo.GetPortfolio(ctx, userID)
	if err != nil {
		log.Printf("[Planner] Portfolio unavailable for user %s: %v", userID, err)
		return nil
	}
	return portfolio
}

func plannerFoodAllowedByProfile(food domain.Food, user *domain.User, portfolio *domain.UserPortfolio) bool {
	if user == nil {
		return !plannerFoodExcludedByPortfolio(food, portfolio)
	}

	terms := strings.ToLower(strings.Join([]string{
		food.Name,
		food.NameEn,
		food.NameVi,
		food.Category,
		food.Source,
	}, " "))

	for _, allergy := range splitPlannerCSV(user.Allergies) {
		if allergy == "" {
			continue
		}
		if strings.Contains(terms, allergy) {
			return false
		}
	}

	diet := strings.ToLower(strings.TrimSpace(user.DietaryPreference))
	switch diet {
	case "vegan":
		if containsPlannerTerm(terms, "beef", "chicken", "fish", "meat", "milk", "pork", "seafood", "shrimp", "egg", "dairy") {
			return false
		}
	case "vegetarian":
		if containsPlannerTerm(terms, "beef", "chicken", "fish", "meat", "pork", "seafood", "shrimp") {
			return false
		}
	case "halal":
		if containsPlannerTerm(terms, "pork", "bacon", "ham", "lard", "alcohol", "wine", "beer") {
			return false
		}
	}

	if plannerFoodExcludedByPortfolio(food, portfolio) {
		return false
	}

	return true
}

func plannerFoodExcludedByPortfolio(food domain.Food, portfolio *domain.UserPortfolio) bool {
	if portfolio == nil {
		return false
	}
	terms := plannerFoodTerms(food)
	for _, excluded := range portfolio.ExcludedIngredients {
		if excluded == "" {
			continue
		}
		if strings.Contains(terms, strings.ToLower(strings.TrimSpace(excluded))) {
			return true
		}
	}
	return false
}

func sortPlannerFoodsByPortfolio(foods []domain.Food, portfolio *domain.UserPortfolio) []domain.Food {
	if portfolio == nil {
		return foods
	}
	sorted := append([]domain.Food(nil), foods...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return plannerPortfolioScore(sorted[i], portfolio) > plannerPortfolioScore(sorted[j], portfolio)
	})
	return sorted
}

func plannerPortfolioScore(food domain.Food, portfolio *domain.UserPortfolio) int {
	score := 0
	terms := plannerFoodTerms(food)
	for _, cuisine := range portfolio.PreferredCuisines {
		if cuisine != "" && strings.Contains(terms, strings.ToLower(strings.TrimSpace(cuisine))) {
			score += 10
		}
	}
	for _, disliked := range portfolio.DislikedIngredients {
		if disliked != "" && strings.Contains(terms, strings.ToLower(strings.TrimSpace(disliked))) {
			score -= 5
		}
	}
	return score
}

func plannerFoodTerms(food domain.Food) string {
	return strings.ToLower(strings.Join([]string{
		food.Name,
		food.NameEn,
		food.NameVi,
		food.Category,
		food.Source,
	}, " "))
}

func splitPlannerCSV(value string) []string {
	parts := strings.Split(strings.ToLower(value), ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		result = append(result, strings.TrimSpace(part))
	}
	return result
}

func containsPlannerTerm(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func plannerTerms(values ...string) []string {
	terms := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			terms = append(terms, value)
		}
	}
	return terms
}

func nonEmptyPlanner(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "Planned meal"
}

func plannerKGUnavailable(resp *domain.AnalyzeMealResponse) bool {
	if resp == nil {
		return false
	}
	if resp.Status != "REJECTED" {
		return false
	}
	if resp.RiskLevel == "UNKNOWN" || resp.HighestRisk == "UNKNOWN" {
		return true
	}
	if len(resp.Violations) == 0 {
		return true
	}
	for _, violation := range resp.Violations {
		description := strings.ToLower(violation.Description)
		severity := strings.ToLower(violation.Severity)
		if strings.Contains(description, "failed to evaluate") ||
			strings.Contains(description, "unavailable") ||
			strings.Contains(severity, "unspecified") ||
			strings.Contains(severity, "unknown") {
			return true
		}
	}
	return false
}

func expandWeeklyPlanMeals(candidates []domain.PlannedMealDTO, startDateRaw string, portfolio *domain.UserPortfolio) []domain.PlannedMealDTO {
	if len(candidates) == 0 {
		return candidates
	}

	startDate, err := time.Parse("2006-01-02", startDateRaw)
	if err != nil {
		startDate = time.Now()
	}

	mealTypes := plannerMealTypesFromPortfolio(portfolio)
	meals := make([]domain.PlannedMealDTO, 0, 21)
	for day := 0; day < 7; day++ {
		date := startDate.AddDate(0, 0, day).Format("2006-01-02")
		for slot, mealType := range mealTypes {
			source := candidates[(day*len(mealTypes)+slot)%len(candidates)]
			meals = append(meals, domain.PlannedMealDTO{
				MealID:   fmt.Sprintf("meal-%s-%s", date, mealType),
				Date:     date,
				FoodIDs:  source.FoodIDs,
				MealType: mealType,
				Status:   source.Status,
				FoodName: source.FoodName,
				Calories: source.Calories,
				Protein:  source.Protein,
				Carbs:    source.Carbs,
				Fat:      source.Fat,
			})
		}
	}
	return meals
}

func plannerMealTypesFromPortfolio(portfolio *domain.UserPortfolio) []string {
	defaultTypes := []string{"breakfast", "lunch", "dinner"}
	if portfolio == nil || portfolio.MealSchedule == nil {
		return defaultTypes
	}
	enabled := make([]string, 0, 4)
	for _, mealType := range []string{"breakfast", "lunch", "dinner", "snack"} {
		if mealScheduleEnabled(portfolio.MealSchedule, mealType) {
			enabled = append(enabled, mealType)
		}
	}
	if len(enabled) == 0 {
		return defaultTypes
	}
	return enabled
}

func mealScheduleEnabled(schedule map[string]any, mealType string) bool {
	raw, ok := schedule[mealType]
	if !ok {
		return mealType != "snack"
	}
	switch value := raw.(type) {
	case bool:
		return value
	case map[string]any:
		if enabled, ok := value["enabled"].(bool); ok {
			return enabled
		}
		return true
	default:
		return true
	}
}

func (u *nutritionUseCase) ReoptimizePlan(ctx context.Context, userID uuid.UUID, req *domain.ReoptimizeWeeklyPlanRequest) (*domain.GenerateWeeklyPlanResponse, error) {
	u.mu.RLock()
	plan, exists := u.planCache[req.PlanID]
	u.mu.RUnlock()

	if !exists {
		if req.CurrentPlan != nil {
			plan = req.CurrentPlan
		} else {
			return nil, fmt.Errorf("plan %s not found in cache", req.PlanID)
		}
	}

	// Create a copy of the plan so we don't mutate the original directly
	newPlan := &domain.WeeklyPlanResponseDTO{
		PlanID: plan.PlanID,
		Meals:  make([]domain.PlannedMealDTO, len(plan.Meals)),
	}
	copy(newPlan.Meals, plan.Meals)

	// Apply adjustment
	if req.Adjustment.Type == "swap_meal" {
		for i, meal := range newPlan.Meals {
			if meal.Date == req.Adjustment.TargetDate && meal.MealType == req.Adjustment.TargetMealType {
				// Analyze the replacement
				candidate := domain.CandidateMeal{
					MealID:   meal.MealID,
					FoodIDs:  []string{req.Adjustment.PreferredFoodID},
					MealType: meal.MealType,
				}

				analyzeReq := &domain.AnalyzeMealRequest{
					Candidate: candidate,
				}

				analyzeResp, err := u.kgPort.AnalyzeMeal(ctx, analyzeReq)
				if err != nil {
					return nil, fmt.Errorf("failed to analyze substitute meal: %w", err)
				}

				if analyzeResp.Status == "REJECTED" {
					return nil, domain.ErrAllCandidatesRejected // Reuse error for safety
				}

				newPlan.Meals[i].FoodIDs = []string{req.Adjustment.PreferredFoodID}
				newPlan.Meals[i].Status = analyzeResp.Status
			}
		}
	}

	u.mu.Lock()
	u.planCache[req.PlanID] = newPlan
	u.mu.Unlock()

	return &domain.GenerateWeeklyPlanResponse{
		WeeklyPlanResponseDTO: *newPlan,
	}, nil
}

func (u *nutritionUseCase) AnalyzeMeal(ctx context.Context, userID uuid.UUID, req *domain.AnalyzeMealRequest) (*domain.AnalyzeMealResponse, error) {
	if u.kgPort == nil {
		if localResp := u.analyzeMealFromCatalog(ctx, userID, req); localResp != nil {
			return localResp, nil
		}
		return nil, fmt.Errorf("KG service unavailable")
	}

	resp, err := u.kgPort.AnalyzeMeal(ctx, req)
	if err != nil {
		if localResp := u.analyzeMealFromCatalog(ctx, userID, req); localResp != nil {
			return localResp, nil
		}
		return nil, err
	}

	normalized := normalizeAnalyzeMealResponse(resp)
	localResp := u.analyzeMealFromCatalog(ctx, userID, req)
	if localResp != nil {
		if len(localResp.Violations) > 0 || mealAnalysisUnavailable(normalized) {
			return localResp, nil
		}
		if normalized.Enrichment == nil {
			normalized.Enrichment = localResp.Enrichment
		}
	}

	return normalized, nil
}

func (u *nutritionUseCase) analyzeMealFromCatalog(ctx context.Context, userID uuid.UUID, req *domain.AnalyzeMealRequest) *domain.AnalyzeMealResponse {
	violations := make([]domain.MealViolation, 0)
	var enrichment *domain.MealEnrichment
	foods := make([]domain.Food, 0, len(req.Candidate.FoodIDs))

	for _, foodID := range req.Candidate.FoodIDs {
		parsedID, err := uuid.Parse(foodID)
		if err != nil {
			return nil
		}
		food, err := u.repo.GetFoodByID(ctx, parsedID)
		if err != nil || food == nil {
			return nil
		}
		foods = append(foods, *food)
	}

	userProfile, _ := u.userRepo.GetByID(ctx, userID)
	userPortfolio := u.loadPlannerPortfolio(ctx, userID)
	for _, food := range foods {
		foodID := food.ID.String()
		foodEnrichment := enrichCatalogFood(food)
		u.persistEstimatedServingSize(ctx, food, foodEnrichment)
		if enrichment == nil {
			enrichment = foodEnrichment
		}
		violations = append(violations, ingredientProfileViolations(foodID, food, foodEnrichment, userProfile)...)

		if plannerFoodExcludedByPortfolio(food, userPortfolio) {
			violations = append(violations, domain.MealViolation{
				ViolationType:    "portfolio_excluded_ingredient",
				Description:      fmt.Sprintf("%s contains an ingredient excluded in your portfolio", nonEmptyPlanner(food.Name, food.NameEn, food.ID.String())),
				Severity:         "CRITICAL",
				OffendingFoodIDs: []string{food.ID.String()},
			})
		}

		if len(foodEnrichment.Ingredients) == 0 && !plannerFoodAllowedByProfile(food, userProfile, userPortfolio) {
			violations = append(violations, domain.MealViolation{
				ViolationType:    "local_profile_constraint",
				Description:      fmt.Sprintf("%s conflicts with your dietary profile", nonEmptyPlanner(food.Name, food.NameEn, food.ID.String())),
				Severity:         "CRITICAL",
				OffendingFoodIDs: []string{food.ID.String()},
			})
		}
	}

	status := "APPROVED"
	safe := true
	risk := "SAFE"
	if len(violations) > 0 {
		status = "REJECTED"
		safe = false
		risk = "CRITICAL"
	}

	return normalizeAnalyzeMealResponse(&domain.AnalyzeMealResponse{
		Status:      status,
		Safe:        safe,
		RiskLevel:   risk,
		HighestRisk: risk,
		Score: domain.MealScore{
			SafetyScore:        boolScore(safe),
			MacroScore:         1,
			MicronutrientScore: 1,
			ConstraintScore:    boolScore(safe),
		},
		Enrichment:       enrichment,
		Violations:       violations,
		Fixes:            []domain.MealFixSuggestion{},
		SafeAlternatives: []domain.MealFixSuggestion{},
	})
}

func (u *nutritionUseCase) persistEstimatedServingSize(ctx context.Context, food domain.Food, enrichment *domain.MealEnrichment) {
	if enrichment == nil || enrichment.EstimatedTotalWeightG <= 100 {
		return
	}
	current := strings.ToLower(strings.TrimSpace(food.ServingSize))
	if current != "" && current != "100g" && current != "100 g" {
		return
	}
	servingSize := fmt.Sprintf("%.0fg", enrichment.EstimatedTotalWeightG)
	if err := u.repo.UpdateFoodServingSize(ctx, food.ID, servingSize); err != nil {
		log.Printf("[MealEnrichment] failed to persist serving size food=%s serving=%s err=%v", food.ID, servingSize, err)
	}
}

func enrichCatalogFood(food domain.Food) *domain.MealEnrichment {
	name := nonEmptyPlanner(food.NameVi, food.Name, food.NameEn)
	terms := strings.ToLower(strings.Join([]string{food.Name, food.NameEn, food.NameVi}, " "))
	terms = strings.ReplaceAll(terms, "phở", "pho")

	enrichment := &domain.MealEnrichment{
		DishName:              name,
		EstimatedTotalWeightG: 100,
		Ingredients:           []domain.MealIngredientEstimate{},
		Source:                "catalog_rule_fallback",
		Confidence:            0.75,
	}

	switch {
	case containsPlannerTerm(terms, "pho thin", "pho bo", "phở bò", "rice noodle with beef"):
		enrichment.DishName = nonEmptyPlanner(food.NameVi, "Pho bo")
		enrichment.EstimatedTotalWeightG = 650
		enrichment.Ingredients = []domain.MealIngredientEstimate{
			{Name: "rice noodles", WeightG: 250},
			{Name: "beef", WeightG: 120},
			{Name: "beef broth", WeightG: 250},
			{Name: "bean sprouts", WeightG: 20},
			{Name: "herbs", WeightG: 10},
		}
		enrichment.Confidence = 0.9
	case containsPlannerTerm(terms, "pho ga", "phở gà", "rice noodle with chicken", "sliced-chicken noodle soup"):
		enrichment.DishName = nonEmptyPlanner(food.NameVi, "Pho ga")
		enrichment.EstimatedTotalWeightG = 650
		enrichment.Ingredients = []domain.MealIngredientEstimate{
			{Name: "rice noodles", WeightG: 250},
			{Name: "chicken", WeightG: 120},
			{Name: "chicken broth", WeightG: 250},
			{Name: "bean sprouts", WeightG: 20},
			{Name: "herbs", WeightG: 10},
		}
		enrichment.Confidence = 0.9
	case containsPlannerTerm(terms, "pho", "phở"):
		enrichment.DishName = nonEmptyPlanner(food.NameVi, food.Name, "Pho")
		enrichment.EstimatedTotalWeightG = 650
		enrichment.Ingredients = []domain.MealIngredientEstimate{
			{Name: "rice noodles", WeightG: 250},
			{Name: "broth", WeightG: 250},
			{Name: "herbs", WeightG: 20},
		}
		enrichment.Confidence = 0.65
	case containsPlannerTerm(terms, "beef"):
		enrichment.Ingredients = []domain.MealIngredientEstimate{{Name: "beef", WeightG: 100}}
	case containsPlannerTerm(terms, "chicken"):
		enrichment.Ingredients = []domain.MealIngredientEstimate{{Name: "chicken", WeightG: 100}}
	case containsPlannerTerm(terms, "pork"):
		enrichment.Ingredients = []domain.MealIngredientEstimate{{Name: "pork", WeightG: 100}}
	case containsPlannerTerm(terms, "shrimp", "shellfish"):
		enrichment.Ingredients = []domain.MealIngredientEstimate{{Name: "shrimp", WeightG: 100}}
	case containsPlannerTerm(terms, "fish", "seafood"):
		enrichment.Ingredients = []domain.MealIngredientEstimate{{Name: "fish", WeightG: 100}}
	}

	return enrichment
}

func ingredientProfileViolations(foodID string, food domain.Food, enrichment *domain.MealEnrichment, user *domain.User) []domain.MealViolation {
	if user == nil || enrichment == nil {
		return nil
	}
	terms := strings.ToLower(strings.Join([]string{food.Name, food.NameEn, food.NameVi, ingredientNames(enrichment.Ingredients)}, " "))
	violations := make([]domain.MealViolation, 0)

	diet := strings.ToLower(strings.TrimSpace(user.DietaryPreference))
	if diet == "vegan" && containsPlannerTerm(terms, "beef", "chicken", "pork", "fish", "shrimp", "seafood", "shellfish", "meat", "egg", "milk", "cheese", "dairy", "broth") {
		violations = append(violations, domain.MealViolation{
			ViolationType:    "diet_vegan",
			Description:      fmt.Sprintf("%s contains animal-derived ingredients: %s", enrichment.DishName, offendingIngredients(enrichment.Ingredients, "beef", "chicken", "pork", "fish", "shrimp", "broth", "egg", "milk", "cheese")),
			Severity:         "CRITICAL",
			OffendingFoodIDs: []string{foodID},
		})
	}
	if diet == "vegetarian" && containsPlannerTerm(terms, "beef", "chicken", "pork", "fish", "shrimp", "seafood", "shellfish", "meat") {
		violations = append(violations, domain.MealViolation{
			ViolationType:    "diet_vegetarian",
			Description:      fmt.Sprintf("%s contains meat or seafood ingredients: %s", enrichment.DishName, offendingIngredients(enrichment.Ingredients, "beef", "chicken", "pork", "fish", "shrimp", "seafood")),
			Severity:         "CRITICAL",
			OffendingFoodIDs: []string{foodID},
		})
	}

	for _, allergy := range splitPlannerCSV(user.Allergies) {
		if allergy == "" {
			continue
		}
		if allergy == "seafood" || allergy == "shellfish" {
			if containsPlannerTerm(terms, allergy, "shrimp", "fish", "crab", "lobster", "clam", "oyster") {
				violations = append(violations, domain.MealViolation{
					ViolationType:    "allergy_" + allergy,
					Description:      fmt.Sprintf("%s may violate your %s allergy", enrichment.DishName, allergy),
					Severity:         "CRITICAL",
					OffendingFoodIDs: []string{foodID},
				})
			}
			continue
		}
		if strings.Contains(terms, allergy) {
			violations = append(violations, domain.MealViolation{
				ViolationType:    "allergy_" + allergy,
				Description:      fmt.Sprintf("%s contains or may contain %s", enrichment.DishName, allergy),
				Severity:         "CRITICAL",
				OffendingFoodIDs: []string{foodID},
			})
		}
	}

	return violations
}

func ingredientNames(ingredients []domain.MealIngredientEstimate) string {
	names := make([]string, 0, len(ingredients))
	for _, ingredient := range ingredients {
		names = append(names, ingredient.Name)
	}
	return strings.Join(names, " ")
}

func offendingIngredients(ingredients []domain.MealIngredientEstimate, needles ...string) string {
	matches := make([]string, 0)
	for _, ingredient := range ingredients {
		lower := strings.ToLower(ingredient.Name)
		if containsPlannerTerm(lower, needles...) {
			matches = append(matches, ingredient.Name)
		}
	}
	if len(matches) == 0 {
		return "animal-derived ingredients"
	}
	return strings.Join(matches, ", ")
}

func boolScore(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func mealAnalysisUnavailable(resp *domain.AnalyzeMealResponse) bool {
	if resp == nil {
		return false
	}
	if resp.RiskLevel == "UNKNOWN" || resp.HighestRisk == "UNKNOWN" {
		return true
	}
	for _, violation := range resp.Violations {
		description := strings.ToLower(violation.Description)
		if strings.Contains(description, "not found in database") ||
			strings.Contains(description, "failed to evaluate food") ||
			strings.Contains(description, "unavailable") {
			return true
		}
	}
	return false
}

func normalizeAnalyzeMealResponse(resp *domain.AnalyzeMealResponse) *domain.AnalyzeMealResponse {
	if resp == nil {
		return resp
	}

	highestRisk := highestMealRisk(resp.Status, resp.Violations)
	resp.HighestRisk = highestRisk
	resp.RiskLevel = highestRisk
	resp.Safe = highestRisk == "SAFE"

	if resp.EvidencePath == nil {
		resp.EvidencePath = make([]domain.MealEvidencePath, 0, len(resp.Violations))
	}
	if len(resp.EvidencePath) == 0 {
		for _, violation := range resp.Violations {
			resp.EvidencePath = append(resp.EvidencePath, domain.MealEvidencePath{
				DiseaseID:       violation.ViolationType,
				RuleDescription: violation.Description,
				Nodes:           evidenceNodesForViolation(violation),
			})
		}
	}

	if resp.Fixes == nil {
		resp.Fixes = []domain.MealFixSuggestion{}
	}
	if resp.SafeAlternatives == nil {
		resp.SafeAlternatives = resp.Fixes
	}

	return resp
}

func highestMealRisk(status string, violations []domain.MealViolation) string {
	highest := riskFromMealStatus(status)
	for _, violation := range violations {
		current := normalizeMealRisk(violation.Severity)
		if riskRank(current) > riskRank(highest) {
			highest = current
		}
	}
	return highest
}

func riskFromMealStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "APPROVED":
		return "SAFE"
	case "WARNING":
		return "WARNING"
	case "REJECTED":
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func normalizeMealRisk(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.TrimPrefix(normalized, "RISK_LEVEL_")
	switch normalized {
	case "CRITICAL", "HIGH", "WARNING", "MODERATE", "LOW", "SAFE":
		return normalized
	case "SEVERE":
		return "CRITICAL"
	case "":
		return "UNKNOWN"
	default:
		return normalized
	}
}

func riskRank(value string) int {
	switch normalizeMealRisk(value) {
	case "CRITICAL":
		return 6
	case "HIGH":
		return 5
	case "WARNING":
		return 4
	case "MODERATE":
		return 3
	case "LOW":
		return 2
	case "SAFE":
		return 1
	default:
		return 0
	}
}

func evidenceNodesForViolation(violation domain.MealViolation) []string {
	nodes := make([]string, 0, len(violation.OffendingFoodIDs)+1)
	nodes = append(nodes, violation.OffendingFoodIDs...)
	if strings.TrimSpace(violation.ViolationType) != "" {
		nodes = append(nodes, violation.ViolationType)
	}
	return nodes
}

var AIFoodRegistry = map[string]string{
	"banh_beo":         "de9a1cbd-27ed-4cf6-84c4-ac23ae49c7ce",
	"banh_bot_loc":     "8c503823-19d0-4248-9f33-f9320e9c4e7d",
	"banh_can":         "3b607f03-496c-4082-abc1-b1836df08638",
	"banh_canh":        "cbe50f45-9654-4799-ab1f-97ec683477c3",
	"banh_chung":       "09982e82-ce84-440d-9c5c-ba70ed20ae38",
	"banh_cuon":        "017ec832-5da8-4a70-9c71-a78263a88b31",
	"banh_duc":         "3f32e1da-c56a-4236-8c5c-0cb4c9e5e178",
	"banh_gio":         "11f2d505-1cc1-4e03-b7cb-028c88aa36a1",
	"banh_khot":        "73b1f6da-170a-4554-9d13-42046c0e14e8",
	"banh_mi":          "a3b7fdae-f6cd-49e9-82f3-f51ec3a534ce",
	"banh_pia":         "e2c3a6e5-aa49-45c7-8b22-bb0dc6349fa2",
	"banh_tet":         "a5a366be-324c-4b40-98e5-df1a4b7e84fc",
	"banh_trang_nuong": "a3b78b44-3191-4d47-9bde-c5b3de226982",
	"banh_xeo":         "df43cbf0-2f5b-4f80-b575-0d28f15c3a49",
	"bun_bo_hue":       "d34e809b-c634-41ab-8bf2-7086506be757",
	"bun_dau_mam_tom":  "c61c7e67-dc2d-4f97-a252-a4e04db2f88a",
	"bun_mam":          "64479a47-d57a-4b19-b2e2-6d7bbe00dbf9",
	"bun_rieu":         "5c543961-42b2-43c4-a821-0cdf74d7691c",
	"bun_thit_nuong":   "70625c41-0f01-4544-9ad3-3f2ef8a27881",
	"ca_kho_to":        "e6c966f0-f6d0-4328-8c5f-fd7c6be44741",
	"canh_chua":        "d54d6238-e299-4074-b980-54aa66dcdda0",
	"cao_lau":          "8a178f5e-d889-4ea0-a727-4824c8f9e80d",
	"chao_long":        "7788269d-2955-428a-9d61-efa6181ca44f",
	"com_tam":          "db17b81e-e5e2-4a49-ae75-022b58169c20",
	"goi_cuon":         "4674dd55-c4d0-43b8-ba94-949abee2e5f2",
	"hu_tieu":          "455231e5-daf8-4a66-a9f3-0fcd305113d7",
	"mi_quang":         "d2e673de-ae0e-4546-b390-f6cfdf85d467",
	"nem_chua":         "42fd673e-11b5-4298-9353-85cb7e4ef371",
	"pho":              "525514d7-4261-4d7d-8cd6-6c5bcf28646f",
	"xoi_xeo":          "25163e9b-a507-4e51-ba5d-e66cdfcf35d0",
}

func mapComplexResultsToFoods(recipes []spoonacular.RecipeResult) []domain.Food {
	foods := make([]domain.Food, 0, len(recipes))
	for _, r := range recipes {
		id := r.ID
		foods = append(foods, domain.Food{
			SpoonacularID: &id, Code: fmt.Sprintf("SPOON-%d", r.ID), Name: r.Title,
			CaloriesPer100g: r.Nutrition.GetNutrient("Calories"), ProteinPer100g: r.Nutrition.GetNutrient("Protein"),
			CarbsPer100g: r.Nutrition.GetNutrient("Carbohydrates"), FatPer100g: r.Nutrition.GetNutrient("Fat"),
			IsVegan: r.Vegan, IsVegetarian: r.Vegetarian, IsGlutenFree: r.GlutenFree, IsDairyFree: r.DairyFree,
			ImageURL: r.Image, ServingSize: fmt.Sprintf("%.0f servings", r.Servings), Source: "Spoonacular",
		})
	}
	return foods
}

func mapNutrientResultsToFoods(results []spoonacular.NutrientSearchResult) []domain.Food {
	foods := make([]domain.Food, 0, len(results))
	for _, r := range results {
		id := r.ID
		foods = append(foods, domain.Food{
			SpoonacularID: &id, Code: fmt.Sprintf("SPOON-%d", r.ID), Name: r.Title,
			CaloriesPer100g: r.Calories, ImageURL: r.Image, Source: "Spoonacular",
		})
	}
	return foods
}

func mapIngredientResultsToFoods(results []spoonacular.IngredientSearchResult) []domain.Food {
	foods := make([]domain.Food, 0, len(results))
	for _, r := range results {
		id := r.ID
		foods = append(foods, domain.Food{
			SpoonacularID: &id, Code: fmt.Sprintf("SPOON-%d", r.ID), Name: r.Title,
			ImageURL: fmt.Sprintf("https://spoonacular.com/recipeImages/%s", r.Image), Source: "Spoonacular",
		})
	}
	return foods
}
