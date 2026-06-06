package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/service"
)

// --- Mock Implementations for Gamification Tests ---

type mockAchievementRepo struct {
	unlocked []domain.UserAchievement
}

func (m *mockAchievementRepo) Unlock(ctx context.Context, userID uuid.UUID, achievementID domain.AchievementID) error {
	for _, ua := range m.unlocked {
		if ua.AchievementID == achievementID {
			return nil // Duplicate unlock
		}
	}
	m.unlocked = append(m.unlocked, domain.UserAchievement{
		UserID:        userID,
		AchievementID: achievementID,
		UnlockedAt:    time.Now(),
	})
	return nil
}

func (m *mockAchievementRepo) IsUnlocked(ctx context.Context, userID uuid.UUID, achievementID domain.AchievementID) (bool, error) {
	for _, ua := range m.unlocked {
		if ua.AchievementID == achievementID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockAchievementRepo) GetUnlocked(ctx context.Context, userID uuid.UUID) ([]domain.UserAchievement, error) {
	return m.unlocked, nil
}

type mockAnalyticsForGamification struct {
	goalHit bool
}

func (m *mockAnalyticsForGamification) BuildAnalyticsRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DayAnalytics, error) {
	return []domain.DayAnalytics{
		{
			GoalHit: m.goalHit,
		},
	}, nil
}

func TestGamificationService_EvaluateAchievements(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()
	now := time.Now()

	t.Run("Unlock streak achievements (1 and 7)", func(t *testing.T) {
		achievementRepo := &mockAchievementRepo{}
		streakRepo := &mockStreakRepo{
			streak: &domain.UserStreak{
				UserID:        userID,
				LongestStreak: 7,
			},
		}
		analyticsSvc := &mockAnalyticsForGamification{goalHit: false}
		userRepo := &mockUserRepoForStreak{timezone: "UTC"}

		svc := service.NewGamificationService(achievementRepo, streakRepo, analyticsSvc, userRepo)
		err := svc.EvaluateAchievements(ctx, userID, domain.TriggerStreakUpdated, now)
		if err != nil {
			t.Fatalf("EvaluateAchievements failed: %v", err)
		}

		unlocked, _ := achievementRepo.GetUnlocked(ctx, userID)
		if len(unlocked) != 2 { // first_goal_hit and streak_7
			t.Errorf("Expected 2 achievements unlocked, got %d: %v", len(unlocked), unlocked)
		}
	})

	t.Run("Unlock streak 30", func(t *testing.T) {
		achievementRepo := &mockAchievementRepo{}
		streakRepo := &mockStreakRepo{
			streak: &domain.UserStreak{
				UserID:        userID,
				LongestStreak: 30,
			},
		}
		analyticsSvc := &mockAnalyticsForGamification{goalHit: false}
		userRepo := &mockUserRepoForStreak{timezone: "UTC"}

		svc := service.NewGamificationService(achievementRepo, streakRepo, analyticsSvc, userRepo)
		svc.EvaluateAchievements(ctx, userID, domain.TriggerStreakUpdated, now)

		unlocked, _ := achievementRepo.GetUnlocked(ctx, userID)
		found := false
		for _, ua := range unlocked {
			if ua.AchievementID == domain.AchievementStreak30 {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AchievementStreak30 not unlocked, got %v", unlocked)
		}
	})

	t.Run("Unlock perfect day", func(t *testing.T) {
		achievementRepo := &mockAchievementRepo{}
		streakRepo := &mockStreakRepo{
			streak: &domain.UserStreak{
				UserID:        userID,
				LongestStreak: 0,
			},
		}
		analyticsSvc := &mockAnalyticsForGamification{goalHit: true}
		userRepo := &mockUserRepoForStreak{timezone: "UTC"}

		svc := service.NewGamificationService(achievementRepo, streakRepo, analyticsSvc, userRepo)
		svc.EvaluateAchievements(ctx, userID, domain.TriggerDailyGoal, now)

		unlocked, _ := achievementRepo.GetUnlocked(ctx, userID)
		if len(unlocked) != 1 || unlocked[0].AchievementID != domain.AchievementPerfectDay {
			t.Errorf("Expected only perfect_day unlocked, got %v", unlocked)
		}
	})

	t.Run("Trigger filtering works", func(t *testing.T) {
		achievementRepo := &mockAchievementRepo{}
		streakRepo := &mockStreakRepo{
			streak: &domain.UserStreak{
				UserID:        userID,
				LongestStreak: 10,
			},
		}
		analyticsSvc := &mockAnalyticsForGamification{goalHit: true}
		userRepo := &mockUserRepoForStreak{timezone: "UTC"}

		svc := service.NewGamificationService(achievementRepo, streakRepo, analyticsSvc, userRepo)
		
		// Trigger ONLY DailyGoal - should not unlock streak achievements even if conditions met
		svc.EvaluateAchievements(ctx, userID, domain.TriggerDailyGoal, now)

		unlocked, _ := achievementRepo.GetUnlocked(ctx, userID)
		if len(unlocked) != 1 || unlocked[0].AchievementID != domain.AchievementPerfectDay {
			t.Errorf("Expected only perfect_day, got %v", unlocked)
		}
	})
}
