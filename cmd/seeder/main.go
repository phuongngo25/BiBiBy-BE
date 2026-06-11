package main

import (
	"log"

	"nutrix-backend/config"
	"nutrix-backend/internal/nutrition/seeder"
	pkgdb "nutrix-backend/pkg/database"
)

func main() {
	log.Println("Starting NutriX Seeder...")

	cfg := config.LoadConfig()

	db, err := pkgdb.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Could not connect to the database: %v", err)
	}

	if err := pkgdb.RunMigrations(db); err != nil {
		log.Fatalf("Could not run migrations: %v", err)
	}

	log.Println("Connected to Database. Starting seed process...")

	if err := seeder.SeedBaseTruthData(db); err != nil {
		log.Fatalf("Seeding failed: %v", err)
	}

	log.Println("Seeding completed successfully!")
}
