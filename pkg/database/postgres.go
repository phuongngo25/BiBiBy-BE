package database

import (
	"fmt"
	"log"
	"nutrix-backend/config"
	"nutrix-backend/internal/domain"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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

	// Clean up legacy workout table
	if err := db.Exec("DROP TABLE IF EXISTS exercise_logs CASCADE;").Error; err != nil {
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

// SeedDummyFoods is a no-op placeholder.
// Replace with actual seeding logic if needed.
func SeedDummyFoods(db *gorm.DB) {
	// Seeding is handled externally via the seeder binary.
}
