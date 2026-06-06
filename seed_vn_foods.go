package main

import (
    "fmt"
    "strings"
    "nutrix-backend/pkg/database"
    "nutrix-backend/config"
    "nutrix-backend/internal/domain"
    "github.com/google/uuid"
)

var vnFoodNames = []string{
    "Banh beo", "Banh bot loc", "Banh can", "Banh canh", "Banh chung", "Banh cuon", "Banh duc", "Banh gio", "Banh khot", "Banh mi", "Banh pia", "Banh tet", "Banh trang nuong", "Banh xeo", "Bun bo Hue", "Bun dau mam tom", "Bun mam", "Bun rieu", "Bun thit nuong", "Ca kho to", "Canh chua", "Cao lau", "Chao long", "Com tam", "Goi cuon", "Hu tieu", "Mi quang", "Nem chua", "Pho", "Xoi xeo",
}

func main() {
    cfg := config.LoadConfig()
    db, err := database.NewPostgresDB(cfg)
    if err != nil {
        panic(err)
    }
    
    fmt.Println("var AIFoodRegistry = map[string]string{")
    for _, name := range vnFoodNames {
        label := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
        
        food := domain.Food{
            ID: uuid.New(),
            Code: label,
            Name: name,
            Source: "AI_Registry",
            CaloriesPer100g: 200.0,
            ProteinPer100g: 10.0,
            FatPer100g: 5.0,
            CarbsPer100g: 25.0,
        }
        
        db.Create(&food)
        fmt.Printf("\t\"%s\": \"%s\",\n", label, food.ID.String())
    }
    fmt.Println("}")
}
