package service

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"nutrix-backend/internal/domain"
)

// GamificationService handles the evaluation and unlocking of achievements.
type GamificationService interface {
	EvaluateAchievements(ctx context.Context, userID uuid.UUID, trigger domain.GamificationTrigger, contextDate time.Time) error
}

type gamificationService struct {
	achievementRepo  domain.AchievementRepository
	streakRepo       domain.StreakRepository
	analyticsService AnalyticsAggregationService
	userRepo         domain.UserRepository
}

// NewGamificationService builds a new GamificationService.
func NewGamificationService(
	achievementRepo domain.AchievementRepository,
	streakRepo domain.StreakRepository,
	analyticsService AnalyticsAggregationService,
	userRepo domain.UserRepository,
) GamificationService {
	return &gamificationService{
		achievementRepo:  achievementRepo,
		streakRepo:       streakRepo,
		analyticsService: analyticsService,
		userRepo:         userRepo,
	}
}

// EvaluateAchievements checks achievement conditions based on a trigger and context date.
func (s *gamificationService) EvaluateAchievements(ctx context.Context, userID uuid.UUID, trigger domain.GamificationTrigger, contextDate time.Time) error {
	// 1. Resolve user timezone
	loc := time.UTC
	userProfile, err := s.userRepo.GetByID(ctx, userID)
	if err == nil && userProfile != nil && userProfile.Timezone != "" {
		if l, errLoc := time.LoadLocation(userProfile.Timezone); errLoc == nil {
			loc = l
		}
	}

	// Normalize contextDate to user's localized midnight
	localTime := contextDate.In(loc)
	targetMidnight := time.Date(localTime.Year(), localTime.Month(), localTime.Day(), 0, 0, 0, 0, loc)

	switch trigger {
	case domain.TriggerStreakUpdated:
		// Check streak-based achievements
		streak, err := s.streakRepo.GetStreak(ctx, userID)
		if err != nil {
			return err
		}
		if streak == nil {
			return nil
		}

		// AchievementFirstGoalHit: Longest streak >= 1
		if streak.LongestStreak >= 1 {
			_ = s.unlockAchievement(ctx, userID, domain.AchievementFirstGoalHit)
		}

		// AchievementStreak7: Longest streak >= 7
		if streak.LongestStreak >= 7 {
			_ = s.unlockAchievement(ctx, userID, domain.AchievementStreak7)
		}

		// AchievementStreak30: Longest streak >= 30
		if streak.LongestStreak >= 30 {
			_ = s.unlockAchievement(ctx, userID, domain.AchievementStreak30)
		}

	case domain.TriggerDailyGoal:
		// Check goal-based achievements for the specific context date
		days, err := s.analyticsService.BuildAnalyticsRange(ctx, userID, targetMidnight, targetMidnight)
		if err != nil {
			return err
		}

		if len(days) > 0 && days[0].GoalHit {
			_ = s.unlockAchievement(ctx, userID, domain.AchievementPerfectDay)
		}
	}

	return nil
}

// unlockAchievement is a helper to unlock an achievement synchronously.
// The repository implementation handles "unlock once" via OnConflict.
func (s *gamificationService) unlockAchievement(ctx context.Context, userID uuid.UUID, achievementID domain.AchievementID) error {
	if err := s.achievementRepo.Unlock(ctx, userID, achievementID); err != nil {
		log.Printf("[GamificationService] ERROR: failed to unlock achievement %s for user %s: %v", achievementID, userID, err)
		return err
	}
	return nil
}
