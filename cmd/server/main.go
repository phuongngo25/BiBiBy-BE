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

	"nutrix-backend/internal/domain"

	nutritionDelivery "nutrix-backend/internal/nutrition/delivery"
	nutritionRepo "nutrix-backend/internal/nutrition/repository"
	nutritionSeeder "nutrix-backend/internal/nutrition/seeder"
	nutritionUC "nutrix-backend/internal/nutrition/usecase"

	userDelivery "nutrix-backend/internal/user/delivery"
	userRepo "nutrix-backend/internal/user/repository"
	userUC "nutrix-backend/internal/user/usecase"

	workoutDelivery "nutrix-backend/internal/workout/delivery"
	workoutRepo "nutrix-backend/internal/workout/repository"
	workoutUC "nutrix-backend/internal/workout/usecase"
	"nutrix-backend/pkg/rapidapi"
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

	// -------------------------------------------------------------------------
	// 2.5 DATABASE SEEDING — Stream JSON directly into DB
	// -------------------------------------------------------------------------
	var foodCount int64
	db.Model(&domain.Food{}).Where("source IN ?", []string{"USDA", "VFA", "VFA_DISH"}).Count(&foodCount)
	if foodCount < 10000 {
		log.Println("Base truth data missing or incomplete, starting auto-seeder...")
		if err := nutritionSeeder.SeedBaseTruthData(db); err != nil {
			log.Printf("Seeder warnings/errors: %v", err)
		}
	} else {
		log.Printf("Database already contains %d food records. Skipping auto-seeder.", foodCount)
	}

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
	// 4.5 DEPENDENCY INJECTION — Workout Context
	// -------------------------------------------------------------------------
	exerciseClient := rapidapi.NewExerciseClient(cfg.RapidAPIKey)
	workoutRepoInst := workoutRepo.NewPostgresWorkoutRepository(db)
	workoutUCInst := workoutUC.NewWorkoutUseCase(workoutRepoInst, exerciseClient)

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
		workoutDelivery.NewWorkoutHandler(protected, workoutUCInst)
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
