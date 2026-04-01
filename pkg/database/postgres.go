package database

import (
	"nutrix-backend/config"
	"nutrix-backend/internal/domain"

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

// RunMigrations enables required PostgreSQL extensions, then runs GORM AutoMigrate
// for all domain models. idempotent — safe to call on every startup.
func RunMigrations(db *gorm.DB) error {
	// pg_trgm enables similarity(), GIN trigram indexes, and the % operator.
	// MUST run before AutoMigrate so the extension exists when indexes are built.
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm;").Error; err != nil {
		return err
	}

	return db.AutoMigrate(
		&domain.User{},
		&domain.Food{},
		&domain.MealLog{},
		&domain.Exercise{},
		&domain.ExerciseLog{},
	)
}

// SeedDummyFoods is a no-op placeholder.
// Replace with actual seeding logic if needed.
func SeedDummyFoods(db *gorm.DB) {
	// Seeding is handled externally via the seeder binary.
}
