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

// GetExercisesByBodyParts acts as a pure proxy to the AscendAPI endpoints.
func (uc *workoutUseCase) GetExercisesByBodyParts(ctx context.Context, bodyParts string) ([]domain.ExerciseListItem, error) {
	if bodyParts == "" {
		return nil, fmt.Errorf("workout: bodyparts parameter is required")
	}

	log.Printf("[workout] Proxying request to AscendAPI for BodyParts=%q", bodyParts)

	fetched, err := uc.exerciseClient.FetchByBodyParts(bodyParts)
	if err != nil {
		return nil, fmt.Errorf("workout: RapidAPI fetch failed: %w", err)
	}

	return fetched, nil
}

// GetExerciseByID acts as a pure proxy to the AscendAPI endpoints.
func (uc *workoutUseCase) GetExerciseByID(ctx context.Context, id string) (*domain.ExerciseDetail, error) {
	if id == "" {
		return nil, fmt.Errorf("workout: id parameter is required")
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

	var calories float64
	if user.WeightKg > 0 {
		// Standard MET formula
		calories = float64(req.DurationMinutes) * (defaultMET * 3.5 * user.WeightKg) / 200.0
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
