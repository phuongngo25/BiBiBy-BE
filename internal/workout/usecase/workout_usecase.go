package usecase

import (
	"context"
	"fmt"
	"log"

	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/rapidapi"
)

// workoutUseCase implements domain.WorkoutUseCase as a direct proxy.
type workoutUseCase struct {
	// repo is kept for interface/DI compatibility if we reinstitute caching later, 
	// but it is completely unused in this proxy iteration.
	repo          domain.WorkoutRepository
	exerciseClient *rapidapi.ExerciseClient
}

// NewWorkoutUseCase wires the repository and RapidAPI client together.
func NewWorkoutUseCase(repo domain.WorkoutRepository, client *rapidapi.ExerciseClient) domain.WorkoutUseCase {
	return &workoutUseCase{
		repo:          repo,
		exerciseClient: client,
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
