// Command seed_demo creates the 20 demo accounts (internal/demoseed.Personas)
// and fills them with a realistic history: meals, water, gym sessions and
// other activities, plus the daily health snapshots, streaks and
// achievements the app derives from them.
//
// Usage (inside the backend image, same env as the server; foods must
// already be seeded by cmd/seeder):
//
//	/app/seed_demo -dry-run         # show what would be written
//	/app/seed_demo                  # create accounts + 60 days of data
//	/app/seed_demo -reset           # wipe and regenerate existing demo data
//
// Only accounts under @demo.bibiby.space are ever touched.
package main

import (
	"context"
	"flag"
	"fmt"
	"hash/fnv"
	"log"
	"math/rand"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"nutrix-backend/config"
	"nutrix-backend/internal/demoseed"
	"nutrix-backend/internal/domain"
	nutritionRepo "nutrix-backend/internal/nutrition/repository"
	nutritionSvc "nutrix-backend/internal/nutrition/service"
	userRepo "nutrix-backend/internal/user/repository"
	pkgdb "nutrix-backend/pkg/database"
)

const timezone = "Asia/Ho_Chi_Minh"

func main() {
	days := flag.Int("days", 60, "days of history, ending today")
	reset := flag.Bool("reset", false, "wipe existing data of demo accounts before regenerating")
	dryRun := flag.Bool("dry-run", false, "generate and print a summary without writing")
	only := flag.String("only", "", "comma-separated usernames to limit the run to (default: all 20)")
	flag.Parse()
	if *days < 1 {
		log.Fatal("-days must be >= 1")
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Fatalf("load timezone: %v", err)
	}

	cfg := config.LoadConfig()
	db, err := pkgdb.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("could not connect to the database: %v", err)
	}

	foods, err := loadFoods(db)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	uRepo := userRepo.NewPostgresUserRepository(db, cfg.EncryptionKeys, cfg.ActiveKeyVersion, cfg.HMACKey)
	nRepo := nutritionRepo.NewPostgresNutritionRepository(db)
	streakRepo := nutritionRepo.NewPostgresStreakRepository(db)
	analytics := nutritionSvc.NewAnalyticsAggregationService(nRepo, uRepo)
	streakSvc := nutritionSvc.NewStreakEvaluationService(nRepo, streakRepo, uRepo, analytics)
	gamification := nutritionSvc.NewGamificationService(nutritionRepo.NewPostgresAchievementRepository(db), streakRepo, analytics, uRepo)

	filter := map[string]bool{}
	for _, u := range strings.Split(*only, ",") {
		if u = strings.TrimSpace(u); u != "" {
			filter[u] = true
		}
	}

	now := time.Now()
	out := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(out, "#\tEMAIL\tPASSWORD\tNAME\tGOAL\tMEALS\tWATER\tWORKOUTS\tSTREAK\tSTATUS")
	failed := false
	for i, p := range demoseed.Personas {
		if len(filter) > 0 && !filter[p.Username] {
			continue
		}
		res, err := seedPersona(ctx, db, i, p, foods, loc, now, *days, *reset, *dryRun)
		status := res.status
		if err != nil {
			status, failed = "FAILED: "+err.Error(), true
		}
		if err == nil && !*dryRun && res.status == "seeded" {
			if s, err := streakSvc.EvaluateStreak(ctx, res.userID, now); err == nil {
				res.streak = fmt.Sprintf("%d (best %d)", s.CurrentStreak, s.LongestStreak)
			}
			_ = gamification.EvaluateAchievements(ctx, res.userID, domain.TriggerStreakUpdated, now)
			for _, snap := range res.history.Snapshots {
				day := time.Date(snap.SnapshotDate.Year(), snap.SnapshotDate.Month(), snap.SnapshotDate.Day(), 12, 0, 0, 0, loc)
				_ = gamification.EvaluateAchievements(ctx, res.userID, domain.TriggerDailyGoal, day)
			}
		}
		h := res.history
		fmt.Fprintf(out, "%d\t%s\t%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\n", i+1, p.Email(), p.Password(i), p.FullName, p.GoalType,
			len(h.Meals), len(h.Water), len(h.Workouts), res.streak, status)
	}
	out.Flush()
	if failed {
		os.Exit(1)
	}
}

// loadFoods maps VFA code → food row for every dish the generator uses.
func loadFoods(db *gorm.DB) (map[string]domain.Food, error) {
	codes := demoseed.AllFoodCodes()
	dishCodes := make([]string, len(codes))
	for i, c := range codes {
		dishCodes[i] = "DISH-" + c
	}
	var rows []domain.Food
	if err := db.Where("code IN ?", dishCodes).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load foods: %w", err)
	}
	foods := make(map[string]domain.Food, len(rows))
	for _, f := range rows {
		foods[strings.TrimPrefix(f.Code, "DISH-")] = f
	}
	if len(foods) < len(codes)/2 {
		return nil, fmt.Errorf("only %d of %d demo dishes found in foods — run the go-seeder (VFA dishes) first", len(foods), len(codes))
	}
	for _, c := range codes {
		if _, ok := foods[c]; !ok {
			log.Printf("WARNING: dish DISH-%s not found, it will not be used", c)
		}
	}
	return foods, nil
}

type result struct {
	userID  uuid.UUID
	history demoseed.History
	streak  string
	status  string
}

func seedPersona(ctx context.Context, db *gorm.DB, idx int, p demoseed.Persona, foods map[string]domain.Food,
	loc *time.Location, now time.Time, days int, reset, dryRun bool) (result, error) {

	res := result{streak: "-"}
	var user domain.User
	err := db.WithContext(ctx).Where("email = ?", p.Email()).First(&user).Error
	exists := err == nil
	if err != nil && err != gorm.ErrRecordNotFound {
		return res, err
	}
	if !exists {
		user.ID = uuid.New()
	}
	res.userID = user.ID

	h := fnv.New64a()
	h.Write([]byte(p.Username))
	res.history = demoseed.Generate(p, user.ID, foods, demoseed.Options{
		Days: days, Now: now, Location: loc, Rng: rand.New(rand.NewSource(int64(h.Sum64()))),
	})
	if dryRun {
		res.status = map[bool]string{true: "dry-run (exists)", false: "dry-run (new)"}[exists]
		return res, nil
	}

	if exists && !reset {
		var n int64
		if err := db.WithContext(ctx).Model(&domain.MealLog{}).Where("user_id = ?", user.ID).Count(&n).Error; err != nil {
			return res, err
		}
		if n > 0 {
			res.status = "skipped (has data, use -reset)"
			return res, nil
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password(idx)), bcrypt.DefaultCost)
	if err != nil {
		return res, err
	}
	dob := p.DOB()
	hist := res.history
	windowStart := now.In(loc).AddDate(0, 0, -days)

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if exists {
			for _, model := range []interface{}{&domain.MealLog{}, &domain.WaterLog{}, &domain.WorkoutLog{},
				&domain.DailyHealthSnapshot{}, &domain.UserStreak{}, &domain.UserAchievement{}} {
				if err := tx.Where("user_id = ?", user.ID).Delete(model).Error; err != nil {
					return err
				}
			}
		}

		user.Username = p.Username
		user.Email = p.Email()
		user.Password = string(hash)
		user.FullName = p.FullName
		user.HeightCm = p.HeightCm
		user.WeightKg = hist.WeightKg
		user.DOB = &dob
		user.Gender = p.Gender
		user.ActivityLevel = p.ActivityLevel
		user.GoalType = p.GoalType
		user.Timezone = timezone
		user.BMR = float64(hist.BMR)
		user.TDEE = float64(hist.TDEE)
		if !exists {
			user.CreatedAt = windowStart
			if err := tx.Create(&user).Error; err != nil {
				return fmt.Errorf("create user: %w", err)
			}
		} else if err := tx.Save(&user).Error; err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		if err := createAll(tx, hist.Snapshots); err != nil {
			return fmt.Errorf("snapshots: %w", err)
		}
		if err := createAll(tx, hist.Meals); err != nil {
			return fmt.Errorf("meals: %w", err)
		}
		if err := createAll(tx, hist.Water); err != nil {
			return fmt.Errorf("water: %w", err)
		}
		if err := createAll(tx, hist.Workouts); err != nil {
			return fmt.Errorf("workouts: %w", err)
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	res.status = "seeded"
	return res, nil
}

func createAll[T any](tx *gorm.DB, rows []T) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.CreateInBatches(rows, 200).Error
}
