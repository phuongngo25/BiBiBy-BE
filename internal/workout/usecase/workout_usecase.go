package usecase

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/service"
	"nutrix-backend/pkg/rapidapi"
)

// workoutUseCase implements domain.WorkoutUseCase.
type workoutUseCase struct {
	repo           domain.WorkoutRepository
	exerciseClient *rapidapi.ExerciseClient
	userRepo       domain.UserRepository // needed to fetch weight_kg for calorie formula
	streakService  service.StreakEvaluationService
}

// NewWorkoutUseCase wires the repository, user repository, RapidAPI client, and streak service.
func NewWorkoutUseCase(
	repo domain.WorkoutRepository,
	client *rapidapi.ExerciseClient,
	userRepo domain.UserRepository,
	streakService service.StreakEvaluationService,
) domain.WorkoutUseCase {
	return &workoutUseCase{
		repo:           repo,
		exerciseClient: client,
		userRepo:       userRepo,
		streakService:  streakService,
	}
}

// GetExercisesByBodyParts returns exercises from RapidAPI when available and
// falls back to the bundled starter catalog when the external provider is down.
func (uc *workoutUseCase) GetExercisesByBodyParts(ctx context.Context, bodyParts string) ([]domain.ExerciseListItem, error) {
	if bodyParts == "" {
		return nil, fmt.Errorf("workout: bodyparts parameter is required")
	}

	log.Printf("[workout] Proxying request to AscendAPI for BodyParts=%q", bodyParts)

	fetched, err := uc.exerciseClient.FetchByBodyParts(bodyParts)
	if err == nil && len(fetched) > 0 {
		return fetched, nil
	}

	// Graceful degradation: only serve the bundled starter catalog when the
	// external provider is unavailable (missing key / network error) or has no
	// match. The live AscendAPI data (with GIFs + heatmaps) is always preferred.
	if err != nil {
		log.Printf("[workout] AscendAPI fetch failed for BodyParts=%q (%v); falling back to local catalog", bodyParts, err)
	} else {
		log.Printf("[workout] AscendAPI returned no exercises for BodyParts=%q; falling back to local catalog", bodyParts)
	}

	if fallback := localExercisesByBodyPart(bodyParts); len(fallback) > 0 {
		log.Printf("[workout] Serving %d local exercises for BodyParts=%q", len(fallback), bodyParts)
		return fallback, nil
	}

	if err != nil {
		return nil, fmt.Errorf("workout: RapidAPI fetch failed: %w", err)
	}
	return fetched, nil
}

// GetMuscleHeatmap proxies the heatmap image so the RapidAPI key stays on the server.
func (uc *workoutUseCase) GetMuscleHeatmap(ctx context.Context, muscle string) ([]byte, string, error) {
	if muscle == "" {
		return nil, "", fmt.Errorf("workout: muscle parameter is required")
	}
	return uc.exerciseClient.FetchHeatmap(muscle)
}

// GetExerciseAsset proxies an exercise image/GIF from the CDN so the browser
// loads it same-origin (the CDN has no CORS headers).
func (uc *workoutUseCase) GetExerciseAsset(ctx context.Context, token string) ([]byte, string, error) {
	if token == "" {
		return nil, "", fmt.Errorf("workout: asset token is required")
	}
	return uc.exerciseClient.FetchAsset(token)
}

// GetExerciseByID returns detail from the bundled starter catalog first, then
// falls back to RapidAPI for IDs that are not part of the local catalog.
func (uc *workoutUseCase) GetExerciseByID(ctx context.Context, id string) (*domain.ExerciseDetail, error) {
	if id == "" {
		return nil, fmt.Errorf("workout: id parameter is required")
	}

	if detail, ok := localExerciseDetailByID(id); ok {
		log.Printf("[workout] Serving local exercise detail for ExerciseID=%q", id)
		return detail, nil
	}

	log.Printf("[workout] Proxying request to AscendAPI for ExerciseID=%q", id)

	fetched, err := uc.exerciseClient.FetchExerciseByID(id)
	if err != nil {
		return nil, fmt.Errorf("workout: RapidAPI fetch failed: %w", err)
	}

	return fetched, nil
}

// LogWorkout calculates calories burned using the MET formula and persists the log.
//
// Formula (Ainsworth Compendium):
//
//	Calories = DurationMinutes × (MET × 3.5 × WeightKg) / 200
//
// Default MET = 5.0 (moderate aerobic exercise).
// Fallback: if user weight is unknown (0), use 8 kcal/min.
func (uc *workoutUseCase) LogWorkout(ctx context.Context, userID uuid.UUID, req *domain.LogWorkoutRequest) (*domain.WorkoutLog, error) {
	const defaultMET = 5.0
	const fallbackKcalPerMin = 8.0

	// Fetch user to get body weight for the MET calculation.
	user, err := uc.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("workout: could not fetch user profile: %w", err)
	}

	// Use the activity's own MET when the client supplies it (e.g. the
	// compendium "Other Activities" flow); otherwise fall back to the default.
	met := defaultMET
	if req.MetValue > 0 {
		met = req.MetValue
	}

	var calories float64
	if user.WeightKg > 0 {
		// Standard MET formula
		calories = float64(req.DurationMinutes) * (met * 3.5 * user.WeightKg) / 200.0
	} else {
		// Graceful fallback when weight has not been set
		calories = float64(req.DurationMinutes) * fallbackKcalPerMin
	}

	// Round to 2 decimal places
	calories = math.Round(calories*100) / 100

	workoutLog := &domain.WorkoutLog{
		UserID:          userID,
		ExerciseID:      req.ExerciseID,
		ExerciseName:    req.ExerciseName,
		DurationMinutes: req.DurationMinutes,
		CaloriesBurned:  calories,
	}

	if err := uc.repo.LogWorkout(ctx, workoutLog); err != nil {
		return nil, fmt.Errorf("workout: failed to save workout log: %w", err)
	}

	log.Printf("[workout] Logged %d min of %q for user %s → %.2f kcal burned",
		req.DurationMinutes, req.ExerciseName, userID, calories)

	if uc.streakService != nil {
		go func() {
			ctxBg, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_, err := uc.streakService.EvaluateStreak(ctxBg, userID, time.Now())
			if err != nil {
				log.Printf("[Streak Hook] WARNING: Streak evaluation failed for user %s on workout: %v", userID, err)
			} else {
				log.Printf("[Streak Hook] Success: Streak evaluated for user %s on workout", userID)
			}
		}()
	}

	return workoutLog, nil
}
