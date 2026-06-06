package usecase

import (
	"context"
	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/service"

	"github.com/google/uuid"
)

type gamificationUseCase struct {
	achievementRepo     domain.AchievementRepository
	gamificationService service.GamificationService
}

// NewGamificationUseCase builds a new GamificationUseCase.
func NewGamificationUseCase(
	achievementRepo domain.AchievementRepository,
	gamificationService service.GamificationService,
) domain.GamificationUseCase {
	return &gamificationUseCase{
		achievementRepo:     achievementRepo,
		gamificationService: gamificationService,
	}
}

// GetAchievements returns all available achievement definitions and the user's unlocked status.
func (u *gamificationUseCase) GetAchievements(ctx context.Context, userID uuid.UUID) (*domain.AchievementResponse, error) {
	// 1. Get unlocked achievements from repo
	unlocked, err := u.achievementRepo.GetUnlocked(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. Map domain models to DTOs
	unlockedDTOs := make([]domain.UnlockedAchievementDTO, len(unlocked))
	for i, ua := range unlocked {
		unlockedDTOs[i] = domain.UnlockedAchievementDTO{
			AchievementID: ua.AchievementID,
			UnlockedAt:    ua.UnlockedAt,
		}
	}

	// 3. Build response with definitions from registry
	return &domain.AchievementResponse{
		Definitions: domain.AchievementRegistry,
		Unlocked:    unlockedDTOs,
	}, nil
}
