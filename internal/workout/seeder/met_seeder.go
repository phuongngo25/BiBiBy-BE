package seeder

import (
	"encoding/json"
	"log"
	"os"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"nutrix-backend/internal/domain"
)

// SeedMetActivities parses met_activities.json and upserts the records into the met_activities table.
func SeedMetActivities(db *gorm.DB, filepath string) error {
	fileBytes, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("MET Seeder Skipped: Could not read file at %s: %v", filepath, err)
		return nil // Non-fatal, just skips
	}

	var activities []domain.MetActivity
	if err := json.Unmarshal(fileBytes, &activities); err != nil {
		log.Printf("MET Seeder Error: Failed to parse JSON: %v", err)
		return err
	}

	if len(activities) == 0 {
		return nil
	}

	// Batch insert with OnConflict to ensure idempotent seeding
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "activity_name"}}, // unique index
		DoUpdates: clause.AssignmentColumns([]string{"met_value", "category"}),
	}).CreateInBatches(activities, 100)

	if result.Error != nil {
		log.Printf("MET Seeder Error: Failed to upsert activities: %v", result.Error)
		return result.Error
	}

	log.Printf("MET Seeder Success: Upserted %d activities", result.RowsAffected)
	return nil
}
