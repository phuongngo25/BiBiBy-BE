package main

import (
	"fmt"
	"nutrix-backend/config"
	"nutrix-backend/internal/domain"
	"nutrix-backend/pkg/database"
)

func main() {
	cfg := config.LoadConfig()
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		panic(err)
	}

	keywords := []string{"pho", "bun", "com"}

	for _, kw := range keywords {
		var foods []domain.Food
		db.Where("name ILIKE ?", "%"+kw+"%").Find(&foods)

		fmt.Printf("--- Searching for '%s' ---\n", kw)
		for _, f := range foods {
			fmt.Printf("ID: %s | Name: %s | Source: %s\n", f.ID.String()[:8], f.Name, f.Source)
		}
		fmt.Println()
	}
}
