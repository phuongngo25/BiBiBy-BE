package demoseed

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/service"
)

// Options controls Generate.
type Options struct {
	Days     int            // days of history, ending today
	Now      time.Time      // nothing is placed after this instant
	Location *time.Location // the user's timezone
	Rng      *rand.Rand
}

// History is everything generated for one user.
type History struct {
	Snapshots []domain.DailyHealthSnapshot
	Meals     []domain.MealLog
	Water     []domain.WaterLog
	Workouts  []domain.WorkoutLog

	// Profile values as of today, to store on the user row.
	WeightKg float64
	BMR      int
	TDEE     int
}

// DOB is the persona's (fixed) date of birth.
func (p Persona) DOB() time.Time { return time.Date(p.BirthYear, time.June, 15, 0, 0, 0, 0, time.UTC) }

// Generate builds a plausible history for one persona.
//
// foods maps VFA codes (without the "DISH-" prefix) to the seeded food rows;
// codes missing from the map are simply not used.
//
// All timestamps fall between 07:00 and 22:00 local time. For UTC+7 that
// keeps the local date equal to the UTC date, which matters because the
// backend aggregates water/workouts with DATE(timestamp) in UTC.
func Generate(p Persona, userID uuid.UUID, foods map[string]domain.Food, opt Options) History {
	loc := opt.Location
	if loc == nil {
		loc = time.UTC
	}
	r := opt.Rng
	now := opt.Now.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	calc := service.NewHealthCalculationService()
	strategy := service.NewGoalStrategyV1()
	dob := p.DOB()

	var h History
	for i := 0; i < opt.Days; i++ {
		day := today.AddDate(0, 0, i-(opt.Days-1))
		at := func(minuteOfDay int) time.Time { return day.Add(time.Duration(minuteOfDay) * time.Minute) }

		weight := round1(p.StartWeightKg + p.WeightPerDay*float64(i) + r.NormFloat64()*0.15)
		bmr := calc.CalculateBMR(weight, p.HeightCm, dob, p.Gender)
		tdee := calc.CalculateTDEE(bmr, p.ActivityLevel)
		h.WeightKg, h.BMR, h.TDEE = weight, bmr, tdee

		// Day 0 is always logged so the streak scan has a first snapshot.
		if i > 0 && r.Float64() > p.LogRate {
			continue
		}

		profile := domain.User{WeightKg: weight, Gender: p.Gender, ActivityLevel: p.ActivityLevel, GoalType: p.GoalType, TDEE: float64(tdee)}
		targetCal, targetWater := strategy.CalculateTargets(&profile)
		utcDate := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		h.Snapshots = append(h.Snapshots, domain.DailyHealthSnapshot{
			UserID: userID, SnapshotDate: utcDate, WeightKg: weight,
			ActivityLevel: p.ActivityLevel, GoalType: p.GoalType, BMR: bmr, TDEE: tdee,
			TargetCalories: targetCal, TargetWater: targetWater, GoalStrategyVersion: "v1",
			CreatedAt: at(7 * 60),
		})

		// ── Meals ────────────────────────────────────────────────────────
		eat := float64(targetCal) * clamp(p.CalorieMean+r.NormFloat64()*p.CalorieSD, 0.55, 1.45)
		hasSnack := r.Float64() < p.SnackRate
		split := map[string]float64{MealBreakfast: 0.27, MealLunch: 0.38, MealDinner: 0.35}
		if hasSnack {
			split = map[string]float64{MealBreakfast: 0.25, MealLunch: 0.34, MealDinner: 0.31, MealSnack: 0.10}
		}
		slots := []struct {
			meal       string
			start, end int // minute-of-day window
		}{
			{MealBreakfast, 7*60 + 10, 8*60 + 40},
			{MealLunch, 11*60 + 30, 12*60 + 50},
			{MealSnack, 15 * 60, 16*60 + 30},
			{MealDinner, 18*60 + 15, 19*60 + 50},
		}
		for _, s := range slots {
			share, ok := split[s.meal]
			if !ok {
				continue
			}
			when := at(s.start + r.Intn(s.end-s.start))
			if when.After(now) {
				continue
			}
			for _, item := range buildMeal(s.meal, eat*share, foods, r) {
				food := foods[item.choice.Code]
				h.Meals = append(h.Meals, domain.MealLog{
					UserID: userID, FoodID: food.ID, ConsumedDate: utcDate, MealType: s.meal,
					QuantityGrams:    round5(item.choice.ServingGrams * item.portions),
					CaloriesConsumed: round2(food.CaloriesPer100g * item.portions),
					ProteinConsumed:  round2(food.ProteinPer100g * item.portions),
					FatConsumed:      round2(food.FatPer100g * item.portions),
					CarbsConsumed:    round2(food.CarbsPer100g * item.portions),
					LoggedAt:         when,
				})
				when = when.Add(time.Duration(1+r.Intn(3)) * time.Minute)
			}
		}

		// ── Water ────────────────────────────────────────────────────────
		drink := int(float64(targetWater) * clamp(p.WaterMean+r.NormFloat64()*0.12, 0.35, 1.4))
		var amounts []int
		for total := 0; total < drink; {
			a := []int{150, 250, 250, 250, 500}[r.Intn(5)]
			amounts = append(amounts, a)
			total += a
		}
		times := make([]int, len(amounts))
		for k := range times {
			times[k] = 7*60 + r.Intn(14*60+30) // 07:00–21:30
		}
		sort.Ints(times)
		for k, a := range amounts {
			when := at(times[k])
			if when.After(now) {
				continue
			}
			src := map[int]string{150: "cup", 250: "glass", 500: "bottle"}[a]
			h.Water = append(h.Water, domain.WaterLog{UserID: userID, AmountMl: a, Source: src, CreatedAt: when})
		}

		// ── Workouts ─────────────────────────────────────────────────────
		session := p.Week[day.Weekday()]
		if session == nil || r.Float64() > p.Adherence {
			continue
		}
		progress := 0.9 + 0.2*float64(i)/math.Max(1, float64(opt.Days-1)) // slowly longer sessions
		start := 17*60 + r.Intn(3*60)                                     // 17:00–20:00
		if p.Morning {
			start = 7*60 + r.Intn(60) // 07:00–08:00
		}
		cursor := at(start)
		var plan []Activity
		var minutes []int
		if len(session.Gym) > 0 {
			n := 3 + r.Intn(len(session.Gym)-2)
			for _, idx := range r.Perm(len(session.Gym))[:n] {
				plan = append(plan, session.Gym[idx])
				minutes = append(minutes, scale(8+r.Intn(8), progress))
			}
		} else {
			plan = append(plan, session.Activity[r.Intn(len(session.Activity))])
			minutes = append(minutes, scale(session.MinMin+r.Intn(session.MaxMin-session.MinMin+1), progress))
		}
		for k, a := range plan {
			if cursor.After(now) {
				break
			}
			h.Workouts = append(h.Workouts, domain.WorkoutLog{
				UserID: userID, ExerciseID: a.ID, ExerciseName: a.Name,
				DurationMinutes: minutes[k],
				CaloriesBurned:  CaloriesBurned(minutes[k], a.MET, weight),
				LoggedAt:        cursor,
			})
			cursor = cursor.Add(time.Duration(minutes[k]+2+r.Intn(3)) * time.Minute)
		}
	}
	return h
}

type mealItem struct {
	choice   FoodChoice
	portions float64 // servings eaten
}

// buildMeal picks dishes for one slot and scales the portion so the slot
// lands near budget kcal (within realistic portion limits).
func buildMeal(meal string, budget float64, foods map[string]domain.Food, r *rand.Rand) []mealItem {
	pick := func(list []FoodChoice) (FoodChoice, bool) {
		for _, k := range r.Perm(len(list)) {
			if f, ok := foods[list[k].Code]; ok && f.CaloriesPer100g > 0 {
				return list[k], true
			}
		}
		return FoodChoice{}, false
	}
	kcal := func(c FoodChoice) float64 { return foods[c.Code].CaloriesPer100g }

	var items []mealItem
	add := func(c FoodChoice, ok bool, portions float64) {
		if ok {
			items = append(items, mealItem{c, portions})
		}
	}
	switch meal {
	case MealBreakfast:
		main, ok := pick(breakfastFoods)
		add(main, ok, 1)
		if ok && budget-kcal(main) > 120 && r.Float64() < 0.6 {
			side, ok := pick(breakfastSides)
			add(side, ok, 1)
		}
	case MealLunch:
		main, ok := pick(lunchFoods)
		add(main, ok, 1)
	case MealDinner:
		bowls := 1.0
		if budget > 650 {
			bowls = 1.5
		}
		if budget > 850 {
			bowls = 2
		}
		_, riceOK := foods[dinnerRice.Code]
		add(dinnerRice, riceOK, bowls)
		main, ok := pick(dinnerMains)
		add(main, ok, 1)
		side, ok := pick(dinnerSides)
		add(side, ok, 1)
	case MealSnack:
		s, ok := pick(snackFoods)
		add(s, ok, 1)
	}

	var base float64
	for _, it := range items {
		base += kcal(it.choice) * it.portions
	}
	if base <= 0 {
		return nil
	}
	factor := clamp(budget/base, 0.6, 1.8)
	for k := range items {
		items[k].portions = math.Round(items[k].portions*factor*20) / 20 // 0.05-serving steps
	}
	return items
}

// CaloriesBurned applies the same formula as workoutUseCase.LogWorkout.
func CaloriesBurned(minutes int, met, weightKg float64) float64 {
	if weightKg <= 0 {
		return round2(float64(minutes) * 8.0)
	}
	return round2(float64(minutes) * (met * 3.5 * weightKg) / 200.0)
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func round1(v float64) float64        { return math.Round(v*10) / 10 }
func round2(v float64) float64        { return math.Round(v*100) / 100 }
func round5(v float64) float64        { return math.Max(5, math.Round(v/5)*5) }
func scale(base int, f float64) int   { return int(math.Max(1, math.Round(float64(base)*f))) }
