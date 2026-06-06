package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"nutrix-backend/config"
	"nutrix-backend/internal/domain"
	pkgdb "nutrix-backend/pkg/database"
)

func TestPostgresAchievementRepository_Unlock(t *testing.T) {
	// 1. Load config and connect to test DB
	_ = godotenv.Load("../../../.env")
	cfg := config.LoadConfig()
	db, err := pkgdb.NewPostgresDB(cfg)
	if err != nil {
		t.Skipf("Skipping integration test: database connection not available: %v", err)
		return
	}

	// Ensure schema exists
	if err := pkgdb.RunMigrations(db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repo := NewPostgresAchievementRepository(db)
	ctx := context.Background()

	// 2. Set up test user
	testUserID := uuid.New()
	user := &domain.User{
		ID:            testUserID,
		Username:      "test_ach_user_" + uuid.New().String()[:8],
		Email:         "test_ach_" + uuid.New().String()[:8] + "@example.com",
		Password:      "secure_hash",
		Gender:        "male",
		ActivityLevel: domain.ActivitySedentary,
		GoalType:      domain.GoalLoseWeight,
		WeightKg:      80,
		HeightCm:      180,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	defer func() {
		// Clean up
		db.Exec("DELETE FROM user_achievements WHERE user_id = ?", testUserID)
		db.Exec("DELETE FROM users WHERE id = ?", testUserID)
	}()

	t.Run("Unlock achievement once", func(t *testing.T) {
		err := repo.Unlock(ctx, testUserID, domain.AchievementFirstGoalHit)
		if err != nil {
			t.Fatalf("Unlock failed: %v", err)
		}

		unlocked, err := repo.GetUnlocked(ctx, testUserID)
		if err != nil {
			t.Fatalf("GetUnlocked failed: %v", err)
		}

		found := false
		for _, ua := range unlocked {
			if ua.AchievementID == domain.AchievementFirstGoalHit {
				found = true
				if ua.UnlockedAt.IsZero() {
					t.Errorf("UnlockedAt should not be zero")
				}
				break
			}
		}
		if !found {
			t.Errorf("AchievementFirstGoalHit not found in unlocked list")
		}
	})

	t.Run("Unlock duplicate (ON CONFLICT DO NOTHING)", func(t *testing.T) {
		// Attempt to unlock the same achievement again
		err := repo.Unlock(ctx, testUserID, domain.AchievementFirstGoalHit)
		if err != nil {
			t.Fatalf("Unlock should not return error on duplicate: %v", err)
		}

		unlocked, _ := repo.GetUnlocked(ctx, testUserID)
		count := 0
		for _, ua := range unlocked {
			if ua.AchievementID == domain.AchievementFirstGoalHit {
				count++
			}
		}
		if count != 1 {
			t.Errorf("Expected exactly 1 record for AchievementFirstGoalHit, got %d", count)
		}
	})

	t.Run("IsUnlocked check", func(t *testing.T) {
		res, err := repo.IsUnlocked(ctx, testUserID, domain.AchievementFirstGoalHit)
		if err != nil || !res {
			t.Errorf("IsUnlocked failed: res=%v, err=%v", res, err)
		}

		res2, err := repo.IsUnlocked(ctx, testUserID, domain.AchievementStreak7)
		if err != nil || res2 {
			t.Errorf("IsUnlocked should be false for streak_7: res=%v, err=%v", res2, err)
		}
	})

	t.Run("Concurrent unlocks", func(t *testing.T) {
		const concurrencyCount = 10
		var wg sync.WaitGroup
		errs := make([]error, concurrencyCount)
		
		for i := 0; i < concurrencyCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				errs[idx] = repo.Unlock(ctx, testUserID, domain.AchievementStreak30)
			}(i)
		}
		wg.Wait()

		for _, err := range errs {
			if err != nil {
				t.Errorf("Concurrent unlock failed: %v", err)
			}
		}

		unlocked, _ := repo.GetUnlocked(ctx, testUserID)
		count := 0
		for _, ua := range unlocked {
			if ua.AchievementID == domain.AchievementStreak30 {
				count++
			}
		}
		if count != 1 {
			t.Errorf("Expected exactly 1 record for AchievementStreak30 after concurrent attempts, got %d", count)
		}
	})
}
