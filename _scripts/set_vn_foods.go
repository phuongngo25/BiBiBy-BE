package main

import (
	"fmt"
	"nutrix-backend/pkg/database"
	"nutrix-backend/config"
	"nutrix-backend/internal/domain"
)

type Macro struct {
	Calories float64
	Protein  float64
	Fat      float64
	Carbs    float64
}

var realisticMacros = map[string]Macro{
	"Banh beo": {150, 3, 2, 30},
	"Banh bot loc": {180, 4, 3, 35},
	"Banh can": {170, 5, 4, 28},
	"Banh canh": {100, 4, 1, 18},
	"Banh chung": {250, 6, 8, 38},
	"Banh cuon": {120, 4, 3, 20},
	"Banh duc": {80, 2, 1, 15},
	"Banh gio": {190, 6, 10, 20},
	"Banh khot": {210, 6, 12, 20},
	"Banh mi": {260, 10, 8, 38},
	"Banh pia": {350, 5, 15, 48},
	"Banh tet": {240, 6, 7, 38},
	"Banh trang nuong": {300, 8, 14, 35},
	"Banh xeo": {220, 7, 12, 21},
	"Bun bo Hue": {110, 6, 4, 13},
	"Bun dau mam tom": {200, 8, 10, 20},
	"Bun mam": {100, 5, 2, 15},
	"Bun rieu": {90, 5, 3, 12},
	"Bun thit nuong": {180, 8, 7, 22},
	"Ca kho to": {160, 14, 9, 5},
	"Canh chua": {40, 2, 1, 6},
	"Cao lau": {150, 6, 4, 22},
	"Chao long": {80, 4, 3, 9},
	"Com tam": {190, 8, 6, 26},
	"Goi cuon": {110, 6, 2, 17},
	"Hu tieu": {100, 5, 2, 15},
	"Mi quang": {140, 6, 5, 18},
	"Nem chua": {150, 18, 6, 4},
	"Pho": {90, 5, 2, 13},
	"Xoi xeo": {280, 6, 8, 45},
}

func main() {
	cfg := config.LoadConfig()
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		panic(err)
	}

	for name, m := range realisticMacros {
		var food domain.Food
		if err := db.Where("name = ?", name).First(&food).Error; err == nil {
			food.CaloriesPer100g = m.Calories
			food.ProteinPer100g = m.Protein
			food.FatPer100g = m.Fat
			food.CarbsPer100g = m.Carbs
			food.Source = "VFA_Estimated" // Mark source clearly
			db.Save(&food)
			fmt.Printf("Updated %s\n", name)
		} else {
			fmt.Printf("Not found: %s\n", name)
		}
	}
}
