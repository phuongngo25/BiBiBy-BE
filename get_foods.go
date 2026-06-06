package main

import (
    "fmt"
    "nutrix-backend/pkg/database"
    "nutrix-backend/config"
    "nutrix-backend/internal/domain"
)

func main() {
    cfg := config.LoadConfig()
    db, _ := database.NewPostgresDB(cfg)
    var foods []domain.Food
    db.Limit(10).Find(&foods)
    for _, f := range foods {
        fmt.Printf("%s - %s\n", f.ID, f.Name)
    }
}
