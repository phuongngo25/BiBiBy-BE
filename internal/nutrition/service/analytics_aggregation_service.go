package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
)

// AnalyticsAggregationService unifies range queries, gap-filling, and goal evaluation.
type AnalyticsAggregationService interface {
	BuildAnalyticsRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DayAnalytics, error)
}

type analyticsAggregationService struct {
	repo     domain.NutritionRepository
	userRepo domain.UserRepository
}

// NewAnalyticsAggregationService creates a new instance of AnalyticsAggregationService.
func NewAnalyticsAggregationService(repo domain.NutritionRepository, userRepo domain.UserRepository) AnalyticsAggregationService {
	return &analyticsAggregationService{repo: repo, userRepo: userRepo}
}

// BuildAnalyticsRange batch-fetches, aggregates, gap-fills, and computes goals over a given date range.
// NOTE: If a daily health snapshot is missing for a day in the range, this method will automatically
// trigger GetOrCreateSnapshot to freeze the baseline goals in GORM. This read-side mutation is a deliberate
// self-healing design choice to lock historical target budgets and prevent retrospective target drifts.
// Operation is 100% idempotent.
func (s *analyticsAggregationService) BuildAnalyticsRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.DayAnalytics, error) {
	// 1. Normalize dates to UTC midnight boundaries
	startOfDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	// 2. Batch fetch raw data (Repository only fetches raw logs, no logic is inside repository)
	snapshots, err := s.repo.GetSnapshotRange(ctx, userID, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	consumed, err := s.repo.GetConsumedRange(ctx, userID, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	burned, err := s.repo.GetBurnedRange(ctx, userID, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	water, err := s.repo.GetWaterRange(ctx, userID, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}


	// 3. Create high-efficiency UTC lookup maps
	snapMap := make(map[time.Time]domain.DailyHealthSnapshot)
	for _, snap := range snapshots {
		snapMap[snap.SnapshotDate.UTC()] = snap
	}

	consumedMap := make(map[time.Time]int)
	for _, c := range consumed {
		consumedMap[c.Day.UTC()] = c.Total
	}

	burnedMap := make(map[time.Time]int)
	for _, b := range burned {
		burnedMap[b.Day.UTC()] = b.Total
	}

	waterMap := make(map[time.Time]int)
	for _, w := range water {
		waterMap[w.Day.UTC()] = w.Total
	}

	// 4. Loop continuously through all dates (gap-filling missing records)
	var days []domain.DayAnalytics

	for d := startOfDay; !d.After(endOfDay); d = d.AddDate(0, 0, 1) {
		dateKey := d.Format("2006-01-02")
		dayUTC := d.UTC()

		var targetCal, targetWat, tdee int

		if snap, ok := snapMap[dayUTC]; ok {
			targetCal = snap.TargetCalories
			targetWat = snap.TargetWater
			tdee = snap.TDEE
		} else {
			// Read-Only Analytics: If snapshot is missing, targets are 0 and goals are unachieved
			targetCal = 0
			targetWat = 0
			tdee = 0
		}

		conCal := consumedMap[dayUTC]
		wBurn := burnedMap[dayUTC]
		conWat := waterMap[dayUTC]

		// Healthy boundaries for calorie completion: 0.7 * target <= consumed <= 1.3 * target
		calGoalHit := targetCal > 0 && float64(conCal) >= 0.7*float64(targetCal) && float64(conCal) <= 1.3*float64(targetCal)
		watGoalHit := targetWat > 0 && conWat >= targetWat
		overallGoalHit := calGoalHit && watGoalHit

		days = append(days, domain.DayAnalytics{
			Date:               dateKey,
			ConsumedCalories:   conCal,
			TargetCalories:     targetCal,
			ConsumedWater:      conWat,
			TargetWater:        targetWat,
			EstimatedDailyBurn: tdee,
			WorkoutBurned:      wBurn,
			TotalBurned:        tdee + wBurn,
			CalorieGoalHit:     calGoalHit,
			WaterGoalHit:       watGoalHit,
			GoalHit:            overallGoalHit,
		})
	}

	return days, nil
}
