package service

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"nutrix-backend/internal/domain"
)

// StreakEvaluationService defines the streak calculation and caching orchestrator.
type StreakEvaluationService interface {
	EvaluateStreak(ctx context.Context, userID uuid.UUID, localizedDay time.Time) (*domain.UserStreak, error)
}

type streakEvaluationService struct {
	repo             domain.NutritionRepository
	streakRepo       domain.StreakRepository
	userRepo         domain.UserRepository
	analyticsService AnalyticsAggregationService
}

// NewStreakEvaluationService builds a new StreakEvaluationService.
func NewStreakEvaluationService(
	repo domain.NutritionRepository,
	streakRepo domain.StreakRepository,
	userRepo domain.UserRepository,
	analyticsService AnalyticsAggregationService,
) StreakEvaluationService {
	return &streakEvaluationService{
		repo:             repo,
		streakRepo:       streakRepo,
		userRepo:         userRepo,
		analyticsService: analyticsService,
	}
}

// EvaluateStreak computes current and longest streak timezone-safely and upserts the derived cache.
func (s *streakEvaluationService) EvaluateStreak(ctx context.Context, userID uuid.UUID, localizedDay time.Time) (*domain.UserStreak, error) {
	// 1. Resolve user timezone
	loc := time.UTC
	userProfile, err := s.userRepo.GetByID(ctx, userID)
	if err == nil && userProfile != nil && userProfile.Timezone != "" {
		if l, errLoc := time.LoadLocation(userProfile.Timezone); errLoc == nil {
			loc = l
		}
	}

	// Convert localizedDay to user's localized location first to prevent UTC off-by-one errors!
	localTime := localizedDay.In(loc)

	// 2. Normalize to local midnight
	todayMidnight := time.Date(localTime.Year(), localTime.Month(), localTime.Day(), 0, 0, 0, 0, loc)

	// 3. Compute Current Streak using unbounded 30-day block backward-scanning
	currentStreak := 0
	
	// Check today's status
	daysToday, err := s.analyticsService.BuildAnalyticsRange(ctx, userID, todayMidnight, todayMidnight)
	if err != nil {
		return nil, err
	}
	todayHit := len(daysToday) > 0 && daysToday[0].GoalHit

	// Check yesterday's status
	yesterday := todayMidnight.AddDate(0, 0, -1)
	daysYesterday, err := s.analyticsService.BuildAnalyticsRange(ctx, userID, yesterday, yesterday)
	if err != nil {
		return nil, err
	}
	yesterdayHit := len(daysYesterday) > 0 && daysYesterday[0].GoalHit

	var scanStartPoint time.Time
	if todayHit {
		scanStartPoint = todayMidnight
	} else if yesterdayHit {
		scanStartPoint = yesterday
	} else {
		scanStartPoint = time.Time{}
	}

	if !scanStartPoint.IsZero() {
		blockEnd := scanStartPoint
		foundBroken := false
		for !foundBroken {
			blockStart := blockEnd.AddDate(0, 0, -29)
			daysBlock, err := s.analyticsService.BuildAnalyticsRange(ctx, userID, blockStart, blockEnd)
			if err != nil {
				return nil, err
			}

			// BuildAnalyticsRange returns days sorted ascending. We scan backwards (right to left).
			for i := len(daysBlock) - 1; i >= 0; i-- {
				if daysBlock[i].GoalHit {
					currentStreak++
				} else {
					foundBroken = true
					break
				}
			}

			if foundBroken {
				break
			}

			// Move blockEnd to the day before blockStart
			blockEnd = blockStart.AddDate(0, 0, -1)
		}
	}

	// 4. Compute Longest Streak using full history scan (guarantees self-healing correctness)
	firstSnapDate, err := s.repo.GetFirstSnapshotDate(ctx, userID)
	if err != nil {
		return nil, err
	}

	// WARNING:
	// V1 intentionally recomputes longest streak from full history
	// to guarantee self-healing correctness.
	//
	// Future optimization:
	// incremental recomputation
	// segmented runs
	// background worker
	longestStreak := 0
	if !firstSnapDate.IsZero() {
		// Convert database UTC snapshot date to the user's localized time representation
		firstLocal := time.Date(firstSnapDate.Year(), firstSnapDate.Month(), firstSnapDate.Day(), 0, 0, 0, 0, loc)
		allDays, err := s.analyticsService.BuildAnalyticsRange(ctx, userID, firstLocal, todayMidnight)
		if err != nil {
			return nil, err
		}

		tempRun := 0
		for _, day := range allDays {
			if day.GoalHit {
				tempRun++
				if tempRun > longestStreak {
					longestStreak = tempRun
				}
			} else {
				tempRun = 0
			}
		}
	}

	// 5. Upsert GORM derived cache
	userStreak := &domain.UserStreak{
		UserID:            userID,
		CurrentStreak:     currentStreak,
		LongestStreak:     longestStreak,
		LastEvaluatedDate: todayMidnight,
	}

	if err := s.streakRepo.UpsertStreak(ctx, userStreak); err != nil {
		log.Printf("[StreakEvaluationService] WARNING: derived cache write failed for user %s: %v", userID, err)
	}

	return userStreak, nil
}
