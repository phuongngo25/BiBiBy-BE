package database

import (
	"encoding/json"
	"fmt"
	"log"
	"nutrix-backend/config"
	"nutrix-backend/internal/domain"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// NewPostgresDB opens a GORM connection to Postgres using the DSN from config.
func NewPostgresDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DBDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// findMigrationFile looks for the migration file by climbing up directories.
// This ensures migrations are found during runtime execution AND go test runs.
func findMigrationFile(filename string) (string, error) {
	if _, err := os.Stat(filename); err == nil {
		return filename, nil
	}
	prefix := ""
	for i := 0; i < 4; i++ {
		prefix += "../"
		path := prefix + filename
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("migration file %s not found in path hierarchy", filename)
}

// RunMigrations enables required PostgreSQL extensions, runs explicit SQL migrations,
// and runs GORM AutoMigrate for other models.
func RunMigrations(db *gorm.DB) error {
	// pg_trgm enables similarity(), GIN trigram indexes, and the % operator.
	// MUST run before AutoMigrate so the extension exists when indexes are built.
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm;").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS unaccent;").Error; err != nil {
		return err
	}

	// Clean up legacy workout table
	if err := db.Exec("DROP TABLE IF EXISTS exercise_logs CASCADE;").Error; err != nil {
		return err
	}

	// Ensure base GORM tables exist before SQL migrations add columns or
	// foreign keys that depend on them. This keeps first boot idempotent.
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Food{},
		&domain.MealLog{},
		&domain.Exercise{},
		&domain.MetActivity{},
		&domain.WorkoutLog{},
		&domain.DRI{},
		&domain.RefreshToken{},
		&domain.WaterLog{},
	); err != nil {
		return err
	}

	// Run Phase 1 explicit SQL migrations
	log.Println("[Migrations] Running explicit migration: 001_add_timezone_to_users.sql")
	migPath1, err := findMigrationFile("migrations/001_add_timezone_to_users.sql")
	if err != nil {
		return err
	}
	migration001, err := os.ReadFile(migPath1)
	if err != nil {
		return err
	}
	if err := db.Exec(string(migration001)).Error; err != nil {
		return err
	}

	log.Println("[Migrations] Running explicit migration: 002_create_daily_health_snapshots.sql")
	migPath2, err := findMigrationFile("migrations/002_create_daily_health_snapshots.sql")
	if err != nil {
		return err
	}
	migration002, err := os.ReadFile(migPath2)
	if err != nil {
		return err
	}
	if err := db.Exec(string(migration002)).Error; err != nil {
		return err
	}

	log.Println("[Migrations] Running explicit migration: 003_create_user_streaks.sql")
	migPath3, err := findMigrationFile("migrations/003_create_user_streaks.sql")
	if err != nil {
		return err
	}
	migration003, err := os.ReadFile(migPath3)
	if err != nil {
		return err
	}
	if err := db.Exec(string(migration003)).Error; err != nil {
		return err
	}

	log.Println("[Migrations] Running explicit migration: 004_create_user_achievements.sql")
	migPath4, err := findMigrationFile("migrations/004_create_user_achievements.sql")
	if err != nil {
		return err
	}
	migration004, err := os.ReadFile(migPath4)
	if err != nil {
		return err
	}
	if err := db.Exec(string(migration004)).Error; err != nil {
		return err
	}

	return db.AutoMigrate(
		&domain.User{},
		&domain.Food{},
		&domain.MealLog{},
		&domain.Exercise{},
		&domain.MetActivity{}, // Added for JSON seeder
		&domain.WorkoutLog{},  // Added for new workout logging feature
		&domain.DRI{},
		&domain.RefreshToken{},
		&domain.WaterLog{}, // Added for hydration tracking feature
	)
}

// SeedDummyFoods populates bundled nutrition data.
// It is idempotent and backfills missing dataset slices independently so a
// partial seed can be repaired without dropping the database.
func SeedDummyFoods(db *gorm.DB) {
	var count int64
	if err := db.Model(&domain.Food{}).Count(&count).Error; err != nil {
		log.Printf("[Seeder] Could not count foods: %v", err)
		return
	}
	if count == 0 {
		foods := make([]domain.Food, 0, 512)
		foods = append(foods, loadVFAFoods("vfa_food_db.json")...)
		foods = append(foods, loadVFADishes("vfa_dishes_db.json")...)
		foods = append(foods, loadUSDAFoods("usda_core_foods.json")...)
		seedFoods(db, foods, "bundled foods")
	} else {
		log.Printf("[Seeder] Food table already has %d rows; checking missing slices", count)
		backfillMissingFoodSource(db, "VFA", "vfa_food_db.json", loadVFAFoods)
		backfillMissingFoodSource(db, "VFA_DISH", "vfa_dishes_db.json", loadVFADishes)
		backfillMissingFoodSource(db, "USDA", "usda_core_foods.json", loadUSDAFoods)
	}

	seedDRIs(db, "DRIs.json")
}

func backfillMissingFoodSource(db *gorm.DB, source, path string, loader func(string) []domain.Food) {
	var count int64
	if err := db.Model(&domain.Food{}).Where("source = ?", source).Count(&count).Error; err != nil {
		log.Printf("[Seeder] Could not count %s foods: %v", source, err)
		return
	}
	if count > 0 {
		log.Printf("[Seeder] %s already has %d rows", source, count)
		return
	}
	seedFoods(db, loader(path), source)
}

func seedFoods(db *gorm.DB, foods []domain.Food, label string) {
	if len(foods) == 0 {
		log.Printf("[Seeder] No %s found to seed", label)
		return
	}

	if err := db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(foods, 500).Error; err != nil {
		log.Printf("[Seeder] Failed to seed %s: %v", label, err)
		return
	}
	log.Printf("[Seeder] Seeded %d %s", len(foods), label)
}

func seedDRIs(db *gorm.DB, path string) {
	var count int64
	if err := db.Model(&domain.DRI{}).Count(&count).Error; err != nil {
		log.Printf("[Seeder] Could not count DRIs: %v", err)
		return
	}
	if count > 0 {
		log.Printf("[Seeder] DRIs already has %d rows", count)
		return
	}

	var dris []domain.DRI
	if !readJSON(path, &dris) {
		return
	}
	if len(dris) == 0 {
		log.Println("[Seeder] No DRIs found to seed")
		return
	}
	if err := db.CreateInBatches(dris, 100).Error; err != nil {
		log.Printf("[Seeder] Failed to seed DRIs: %v", err)
		return
	}
	log.Printf("[Seeder] Seeded %d DRIs", len(dris))
}

func loadVFAFoods(path string) []domain.Food {
	type nutrient struct {
		NameEn string  `json:"name_en"`
		Value  float64 `json:"value"`
	}
	type item struct {
		Code      string     `json:"code"`
		NameVI    string     `json:"name_vi"`
		NameEN    string     `json:"name_en"`
		Category  string     `json:"categoryEn"`
		Nutrition []nutrient `json:"nutrition"`
	}

	var items []item
	if !readJSON(path, &items) {
		return nil
	}

	foods := make([]domain.Food, 0, len(items))
	for _, item := range items {
		food := domain.Food{
			Code:        nonEmpty("vfa_food_"+item.Code, "vfa_food_"+slug(item.NameEN)),
			Name:        nonEmpty(item.NameEN, item.NameVI),
			NameVi:      item.NameVI,
			NameEn:      nonEmpty(item.NameEN, item.NameVI),
			Category:    item.Category,
			Source:      "VFA",
			ServingSize: "100g",
			IsVerified:  true,
		}
		for _, n := range item.Nutrition {
			applyNutrient(&food, n.NameEn, n.Value)
		}
		if food.NameEn != "" {
			foods = append(foods, food)
		}
	}
	return foods
}

func loadVFADishes(path string) []domain.Food {
	type component struct {
		NameEn string `json:"nameEn"`
		Amount any    `json:"amount"`
	}
	type item struct {
		Code                  string      `json:"code"`
		NameVI                string      `json:"name_vi"`
		NameEN                string      `json:"name_en"`
		NutritionalComponents []component `json:"nutritional_components"`
	}

	var items []item
	if !readJSON(path, &items) {
		return nil
	}

	foods := make([]domain.Food, 0, len(items))
	for _, item := range items {
		food := domain.Food{
			Code:        nonEmpty("vfa_dish_"+item.Code, "vfa_dish_"+slug(item.NameEN)),
			Name:        nonEmpty(item.NameEN, item.NameVI),
			NameVi:      item.NameVI,
			NameEn:      nonEmpty(item.NameEN, item.NameVI),
			Category:    "Dish",
			Source:      "VFA_DISH",
			ServingSize: "100g",
			IsVerified:  true,
		}
		for _, n := range item.NutritionalComponents {
			applyNutrient(&food, n.NameEn, parseFloat(n.Amount))
		}
		if food.NameEn != "" {
			foods = append(foods, food)
		}
	}
	return foods
}

func parseFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case string:
		var out float64
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%f", &out); err == nil {
			return out
		}
	}
	return 0
}

func loadUSDAFoods(path string) []domain.Food {
	type nutrient struct {
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
	}
	type item struct {
		FDCID         int        `json:"fdcId"`
		Description   string     `json:"description"`
		DataType      string     `json:"dataType"`
		FoodNutrients []nutrient `json:"foodNutrients"`
	}

	var items []item
	if !readJSON(path, &items) {
		return nil
	}

	foods := make([]domain.Food, 0, len(items))
	for _, item := range items {
		food := domain.Food{
			Code:        fmt.Sprintf("usda_%d", item.FDCID),
			Name:        item.Description,
			NameEn:      item.Description,
			Category:    item.DataType,
			Source:      "USDA",
			ServingSize: "100g",
			IsVerified:  true,
		}
		for _, n := range item.FoodNutrients {
			applyNutrient(&food, n.Name, n.Amount)
		}
		if food.NameEn != "" {
			foods = append(foods, food)
		}
	}
	return foods
}

func readJSON(path string, out any) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[Seeder] Skipping %s: %v", path, err)
		return false
	}
	if err := json.Unmarshal(data, out); err != nil {
		log.Printf("[Seeder] Could not parse %s: %v", path, err)
		return false
	}
	return true
}

func applyNutrient(food *domain.Food, name string, value float64) {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "energy"):
		food.CaloriesPer100g = value
	case strings.Contains(n, "protein"):
		food.ProteinPer100g = value
	case strings.Contains(n, "carbohydrate") || strings.Contains(n, "carb"):
		food.CarbsPer100g = value
	case strings.Contains(n, "lipid") || strings.Contains(n, "fat"):
		food.FatPer100g = value
	}
}

func nonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ",", "", "'", "")
	value = replacer.Replace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
