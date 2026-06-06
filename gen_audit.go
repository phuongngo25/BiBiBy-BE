package main

import (
	"fmt"
	"nutrix-backend/pkg/database"
	"nutrix-backend/config"
	"nutrix-backend/internal/domain"
	"nutrix-backend/internal/nutrition/usecase"
	"sort"
)

func main() {
	cfg := config.LoadConfig()
	db, _ := database.NewPostgresDB(cfg)

	fmt.Println("| AI Label | Food ID | Food Name | Source | Calories/100g | Protein/100g | Fat/100g | Carbs/100g |")
	fmt.Println("|---|---|---|---|---|---|---|---|")

	var keys []string
	for k := range usecase.AIFoodRegistry {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		id := usecase.AIFoodRegistry[k]
		var f domain.Food
		if err := db.Where("id = ?", id).First(&f).Error; err == nil {
			fmt.Printf("| %s | %s | %s | %s | %.0f | %.1f | %.1f | %.1f |\n", k, id, f.Name, f.Source, f.CaloriesPer100g, f.ProteinPer100g, f.FatPer100g, f.CarbsPer100g)
		} else {
			fmt.Printf("| %s | %s | NOT FOUND | - | - | - | - | - |\n", k, id)
		}
	}
}
