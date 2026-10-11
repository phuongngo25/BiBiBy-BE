package demoseed

import (
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"

	"nutrix-backend/internal/domain"
)

// fakeFoods gives every catalog code a plausible per-serving value.
func fakeFoods() map[string]domain.Food {
	m := map[string]domain.Food{}
	for i, c := range AllFoodCodes() {
		m[c] = domain.Food{ID: uuid.New(), Code: "DISH-" + c, CaloriesPer100g: float64(150 + 25*(i%16)), ProteinPer100g: 15, CarbsPer100g: 40, FatPer100g: 10}
	}
	m[dinnerRice.Code] = domain.Food{ID: uuid.New(), CaloriesPer100g: 258, CarbsPer100g: 56}
	return m
}

func TestGenerateHistory(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Skip("tzdata unavailable:", err)
	}
	now := time.Date(2026, 10, 11, 23, 30, 0, 0, loc)
	foods := fakeFoods()

	for idx, p := range Personas {
		uid := uuid.New()
		h := Generate(p, uid, foods, Options{Days: 60, Now: now, Location: loc, Rng: rand.New(rand.NewSource(int64(idx)))})

		if len(h.Snapshots) < 30 || len(h.Meals) == 0 || len(h.Water) == 0 || len(h.Workouts) == 0 {
			t.Fatalf("%s: thin history: %d snapshots, %d meals, %d water, %d workouts", p.Username, len(h.Snapshots), len(h.Meals), len(h.Water), len(h.Workouts))
		}
		if h.TDEE <= 0 || h.WeightKg <= 0 {
			t.Fatalf("%s: bad profile %+v", p.Username, h)
		}

		// Every timestamp is local 07:00–22:00 → same calendar date in UTC.
		check := func(kind string, ts time.Time) {
			l := ts.In(loc)
			if l.After(now) || l.Hour() < 7 || l.Hour() >= 22 || l.Format("2006-01-02") != ts.UTC().Format("2006-01-02") {
				t.Fatalf("%s: %s at bad time %v", p.Username, kind, l)
			}
		}

		daily := map[string]float64{}
		for _, m := range h.Meals {
			check("meal", m.LoggedAt)
			if m.UserID != uid || m.QuantityGrams <= 0 || m.CaloriesConsumed <= 0 {
				t.Fatalf("%s: bad meal %+v", p.Username, m)
			}
			daily[m.ConsumedDate.Format("2006-01-02")] += m.CaloriesConsumed
		}
		for _, w := range h.Water {
			check("water", w.CreatedAt)
		}
		for _, w := range h.Workouts {
			check("workout", w.LoggedAt)
			if w.CaloriesBurned <= 0 || w.DurationMinutes <= 0 {
				t.Fatalf("%s: bad workout %+v", p.Username, w)
			}
		}

		// Average intake should track the targets (personas aim near 1.0).
		var sumRatio float64
		var n int
		for _, s := range h.Snapshots {
			if c, ok := daily[s.SnapshotDate.Format("2006-01-02")]; ok && s.SnapshotDate.Before(now.Add(-24*time.Hour)) {
				sumRatio += c / float64(s.TargetCalories)
				n++
			}
		}
		if avg := sumRatio / float64(n); avg < 0.7 || avg > 1.3 {
			t.Fatalf("%s: avg intake/target = %.2f", p.Username, avg)
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	now := time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)
	uid := uuid.New()
	foods := fakeFoods()
	a := Generate(Personas[0], uid, foods, Options{Days: 14, Now: now, Rng: rand.New(rand.NewSource(7))})
	b := Generate(Personas[0], uid, foods, Options{Days: 14, Now: now, Rng: rand.New(rand.NewSource(7))})
	if len(a.Meals) != len(b.Meals) || a.Meals[3].CaloriesConsumed != b.Meals[3].CaloriesConsumed || len(a.Workouts) != len(b.Workouts) {
		t.Fatal("same seed produced different histories")
	}
}

func TestPersonasAreValid(t *testing.T) {
	seen := map[string]bool{}
	for i, p := range Personas {
		if seen[p.Email()] {
			t.Fatalf("duplicate email %s", p.Email())
		}
		seen[p.Email()] = true
		pw := p.Password(i)
		if len(pw) < 8 {
			t.Fatalf("weak password %q", pw)
		}
		for wd, s := range p.Week {
			if s != nil && len(s.Gym) == 0 && (len(s.Activity) == 0 || s.MaxMin < s.MinMin) {
				t.Fatalf("%s: bad session on %v", p.Username, time.Weekday(wd))
			}
			if s != nil && len(s.Gym) > 0 && len(s.Gym) < 3 {
				t.Fatalf("%s: gym pool needs 3+ exercises", p.Username)
			}
		}
	}
	if len(Personas) != 20 {
		t.Fatalf("expected 20 personas, got %d", len(Personas))
	}
}

func TestCaloriesBurnedMatchesUsecaseFormula(t *testing.T) {
	if got := CaloriesBurned(30, 5, 70); got != 183.75 { // 30 × (5 × 3.5 × 70) / 200
		t.Fatalf("got %v", got)
	}
	if got := CaloriesBurned(10, 5, 0); got != 80 {
		t.Fatalf("fallback: got %v", got)
	}
}
