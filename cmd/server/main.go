package main

import (
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"nutrix-backend/config"
	pkgdb "nutrix-backend/pkg/database"
	"nutrix-backend/pkg/middleware"
	"nutrix-backend/pkg/spoonacular"

	nutritionDelivery "nutrix-backend/internal/nutrition/delivery"
	nutritionRepo "nutrix-backend/internal/nutrition/repository"
	nutritionUC "nutrix-backend/internal/nutrition/usecase"

	userDelivery "nutrix-backend/internal/user/delivery"
	userRepo "nutrix-backend/internal/user/repository"
	userUC "nutrix-backend/internal/user/usecase"
)

func main() {
	// -------------------------------------------------------------------------
	// 1. CONFIGURATION — Load env vars from .env (or system env)
	// -------------------------------------------------------------------------
	cfg := config.LoadConfig()

	// -------------------------------------------------------------------------
	// 2. DATABASE — Centralized GORM connection with AutoMigrate
	// -------------------------------------------------------------------------
	db, err := pkgdb.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Could not initialize database: %v", err)
	}

	// Run SQL migrations to ensure all tables exist (idempotent — safe on every startup)
	if err := pkgdb.RunMigrations(db); err != nil {
		log.Fatalf("Could not run database migrations: %v", err)
	}

	// Run seeder to populate food data if DB is empty
	pkgdb.SeedDummyFoods(db)

	// -------------------------------------------------------------------------
	// 3. DEPENDENCY INJECTION — Nutrition Context
	// -------------------------------------------------------------------------
	spoonClient := spoonacular.NewClient(cfg.SpoonacularAPIKey)
	nutritionRepoInst := nutritionRepo.NewPostgresNutritionRepository(db)
	nutritionUCInst := nutritionUC.NewNutritionUseCase(nutritionRepoInst, spoonClient)

	// -------------------------------------------------------------------------
	// 4. DEPENDENCY INJECTION — User Context
	// -------------------------------------------------------------------------
	uRepo := userRepo.NewPostgresUserRepository(db)
	uUC := userUC.NewUserUseCase(uRepo, cfg.JWTSecret, cfg.JWTExpirationHours)

	// -------------------------------------------------------------------------
	// 5. HTTP ROUTER & SECURITY MIDDLEWARES
	// -------------------------------------------------------------------------
	r := gin.Default()

	// Handle Cross-Origin restrictions globally
	configCORS := cors.DefaultConfig()
	configCORS.AllowAllOrigins = true // In prod, set to specific domains (e.g., flutter web domain)
	configCORS.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(configCORS))

	// Global Security: Stop large payloads (5MB max) and Brute-force requests (100 req/min/IP)
	r.Use(middleware.MaxBodySize(5 * 1024 * 1024))
	r.Use(middleware.RateLimiter())

	// Health check endpoint
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "NutriX is healthy!"})
	})

	// Serve uploaded food images as static files: GET /uploads/foods/{filename}
	r.Static("/uploads", "./uploads")

	// -------------------------------------------------------------------------
	// 6. DELIVERY HANDLERS — Register routes
	// Auth routes (public) — no JWT required
	userDelivery.NewUserHandler(r, uUC)

	// Protected routes example — wrap with JWT middleware
	protected := r.Group("/api/v1")
	protected.Use(middleware.RequireAuth(cfg.JWTSecret))
	{
		nutritionDelivery.NewNutritionHandler(protected, nutritionUCInst)
		userDelivery.RegisterProfileRoutes(protected, uUC)
	}
	// -------------------------------------------------------------------------

	// -------------------------------------------------------------------------
	// 7. START SERVER
	// -------------------------------------------------------------------------
	log.Printf("NutriX Server starting on port :%s\n", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
