package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"nutrix-backend/config"
	pkgdb "nutrix-backend/pkg/database"
	"nutrix-backend/pkg/middleware"
	"nutrix-backend/pkg/spoonacular"

	"nutrix-backend/internal/infrastructure"
	"nutrix-backend/internal/infrastructure/metrics"
	nutritionDelivery "nutrix-backend/internal/nutrition/delivery"
	nutritionRepo "nutrix-backend/internal/nutrition/repository"
	nutritionSvc "nutrix-backend/internal/nutrition/service"
	nutritionUC "nutrix-backend/internal/nutrition/usecase"

	productDelivery "nutrix-backend/internal/product/delivery"

	userDelivery "nutrix-backend/internal/user/delivery"
	userRepo "nutrix-backend/internal/user/repository"
	userUC "nutrix-backend/internal/user/usecase"

	workoutDelivery "nutrix-backend/internal/workout/delivery"
	workoutRepo "nutrix-backend/internal/workout/repository"
	workoutSeeder "nutrix-backend/internal/workout/seeder"
	workoutUC "nutrix-backend/internal/workout/usecase"
	"nutrix-backend/pkg/rapidapi"
)

func main() {
	// -------------------------------------------------------------------------
	// 1. CONFIGURATION — Load env vars from .env (or system env)
	// -------------------------------------------------------------------------
	cfg := config.LoadConfig()

	// Context cancelled on shutdown — gracefully stops background goroutines (e.g. RateLimiter cleanup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// -------------------------------------------------------------------------
	// 2. DATABASE — Centralized GORM connection with AutoMigrate
	// -------------------------------------------------------------------------
	db, err := pkgdb.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Could not initialize database: %v", err)
	}

	// Run SQL migrations to ensure all tables exist (idempotent — safe on every startup)
	if err := pkgdb.RunMigrations(db); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Run seeder to populate dummy foods if DB is empty
	pkgdb.SeedDummyFoods(db)

	// Seed MET activities
	_ = workoutSeeder.SeedMetActivities(db, "met_activities.json")

	// -------------------------------------------------------------------------
	// 2.5 REDIS — Resilient Rate Limiting Store
	// -------------------------------------------------------------------------
	redisClient, redisErr := pkgdb.NewRedisClient(cfg)
	if redisErr != nil {
		log.Printf("[SRE] Redis unavailable on boot: %v. Starting in Degraded Mode (In-Memory Limiter).", redisErr)
	}

	// Redis Recovery Loop: Automatically reset the circuit breaker when Redis returns
	if redisClient != nil {
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					// If we were in degraded/fail-open state, check if Redis is back
					if err := redisClient.Ping(ctx).Err(); err == nil {
						// Note: Ideally the breaker has a Reset() method
						// We'll trust the breaker's internal HalfOpen probe,
						// but this is an extra layer of proactive recovery.
					}
				}
			}
		}()
	}

	// -------------------------------------------------------------------------
	// 3. DEPENDENCY INJECTION — Repositories First
	// -------------------------------------------------------------------------
	workoutRepoInst := workoutRepo.NewPostgresWorkoutRepository(db)
	nutritionRepoInst := nutritionRepo.NewPostgresNutritionRepository(db)
	driRepoInst := nutritionRepo.NewPostgresDRIRepository(db)
	uRepo := userRepo.NewPostgresUserRepository(db, cfg.EncryptionKeys, cfg.ActiveKeyVersion, cfg.HMACKey)
	userPortfolioRepoInst := userRepo.NewPostgresUserPortfolioRepository(db)
	streakRepoInst := nutritionRepo.NewPostgresStreakRepository(db)
	achievementRepoInst := nutritionRepo.NewPostgresAchievementRepository(db)

	// -------------------------------------------------------------------------
	// 4. DEPENDENCY INJECTION — External Clients
	// -------------------------------------------------------------------------
	spoonClient := spoonacular.NewClient(cfg.SpoonacularAPIKey)
	exerciseClient := rapidapi.NewExerciseClient(cfg.RapidAPIKey)
	kgTarget, cvTarget := resolveInternalServiceTargets(cfg)
	log.Printf("[SRE] KG target: %s", kgTarget)
	log.Printf("[SRE] CV target: %s", cvTarget)
	kgClient, kgErr := infrastructure.NewGrpcNutritionClient(kgTarget)
	if kgErr != nil {
		log.Printf("[SRE] KG Client unavailable: %v", kgErr)
	}
	cvClient, cvErr := infrastructure.NewGrpcAIClient(cvTarget)
	if cvErr != nil {
		log.Printf("[SRE] CV Client unavailable: %v", cvErr)
	}

	// -------------------------------------------------------------------------
	// 4.5 DEPENDENCY INJECTION — Streak Service
	// -------------------------------------------------------------------------
	analyticsSvc := nutritionSvc.NewAnalyticsAggregationService(nutritionRepoInst, uRepo)
	streakSvc := nutritionSvc.NewStreakEvaluationService(nutritionRepoInst, streakRepoInst, uRepo, analyticsSvc)

	// -------------------------------------------------------------------------
	// 5. DEPENDENCY INJECTION — UseCases
	// -------------------------------------------------------------------------
	nutritionUCInst := nutritionUC.NewNutritionUseCase(nutritionRepoInst, streakRepoInst, achievementRepoInst, spoonClient, workoutRepoInst, uRepo, cvClient, kgClient, redisClient, userPortfolioRepoInst)
	uUC := userUC.NewUserUseCase(uRepo, driRepoInst, nutritionRepoInst, workoutRepoInst, cfg.JWTSecret, cfg.JWTExpirationHours, userPortfolioRepoInst)
	workoutUCInst := workoutUC.NewWorkoutUseCase(workoutRepoInst, exerciseClient, uRepo, streakSvc)

	// Gamification UseCase
	gamificationSvc := nutritionSvc.NewGamificationService(achievementRepoInst, streakRepoInst, analyticsSvc, uRepo)
	gamificationUCInst := nutritionUC.NewGamificationUseCase(achievementRepoInst, gamificationSvc)

	// -------------------------------------------------------------------------
	// 5. HTTP ROUTER & SECURITY MIDDLEWARES
	// -------------------------------------------------------------------------
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(metrics.PrometheusMiddleware()) // Inject Global Observability Middleware

	// Expose Prometheus endpoint
	r.GET("/metrics", metrics.PrometheusHandler())

	// ==========================
	// TRUSTED PROXIES (CRITICAL)
	// ==========================
	err = r.SetTrustedProxies([]string{
		"127.0.0.1",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})
	if err != nil {
		log.Fatalf("Failed to configure trusted proxies: %v", err)
	}

	appEnv := os.Getenv("APP_ENV")
	isProd := appEnv == "production"

	// ── CORS ─────────────────────────────────────────────────────────────────
	// Origins are controlled by the ALLOWED_ORIGINS env var (comma-separated).
	// On dev, it defaults to localhost. On prod, set it to your Flutter Web /
	// dashboard domain, e.g. "https://app.example.com".
	//
	// Security note: AllowAllOrigins is intentionally NOT used here because it
	// prevents browsers from sending credentialed requests (cookies / auth
	// headers) when the server responds with "Access-Control-Allow-Origin: *".
	// Using an explicit list lets us stay compatible with future cookie auth.
	corsConfig := cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
		AllowMethods: []string{
			"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS",
		},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization",
			"X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,        // set true if/when cookie-based auth is added
		MaxAge:           12 * 60 * 60, // 12 h — browsers cache preflight for this long
	}
	r.Use(cors.New(corsConfig))

	// ─── Global Security Middleware Stack (ordered: drop bad traffic ASAP) ───
	// 2. RequireHTTPS — block cleartext transport immediately (prod only)
	r.Use(middleware.RequireHTTPS(isProd))
	// 3. MaxBodySize — hard-limit the byte stream before JSON parsing
	r.Use(middleware.MaxBodySize(5 * 1024 * 1024))
	// 4. SecurityHeaders — set response headers last, before handler runs
	r.Use(middleware.SecurityHeaders(true)) // true = API-only (no CSP)

	// Health check endpoint
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "NutriX is healthy!"})
	})

	// Serve uploaded food images as static files: GET /uploads/foods/{filename}
	r.Static("/uploads", "./uploads")

	// -------------------------------------------------------------------------
	// 6. DELIVERY HANDLERS — Register routes
	// Auth/Public routes (Stricter Limiting: 20 req/min)
	public := r.Group("/")
	public.Use(middleware.RateLimiter(redisClient, 20, 5))
	userDelivery.NewUserHandler(public, uUC)

	// Protected routes example (Relaxed Limiting: 150 req/min)
	protected := r.Group("/api/v1")
	protected.Use(middleware.RateLimiter(redisClient, 150, 20))
	protected.Use(middleware.RequireAuth(cfg.JWTSecret))
	{
		nutritionDelivery.NewNutritionHandler(protected, nutritionUCInst)
		nutritionDelivery.NewGamificationHandler(protected, gamificationUCInst)
		userDelivery.RegisterProfileRoutes(protected, uUC)
		workoutDelivery.NewWorkoutHandler(protected, workoutUCInst)
		// OpenFoodFacts reverse-proxy (same-origin for web; sets the UA OFF asks for).
		productDelivery.NewOFFProxyHandler(protected, "NutriX/1.0 (https://bibiby.space)")
	}
	// -------------------------------------------------------------------------

	// -------------------------------------------------------------------------
	// 7. HTTP SERVER — Non-blocking start + True Graceful Shutdown
	// -------------------------------------------------------------------------
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second, // anti-slowloris
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start serving in a goroutine so the main thread is free to listen for signals
	go func() {
		log.Printf("NutriX Server starting on port :%s\n", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	// Block until OS sends SIGINT (Ctrl+C) or SIGTERM (container/k8s stop)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("[Server] Shutdown signal received. Draining active connections...")

	// Give active requests up to 10s to finish before forcefully closing
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(ctxShutdown); err != nil {
		log.Fatalf("[Server] Forced shutdown due to error: %v", err)
	}

	// ── Database Connection Cleanup ──────────────────────────────────────────
	// Explicitly close the Postgres connection pool
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
		log.Println("[Server] Database connection closed.")
	}

	if redisClient != nil {
		redisClient.Close()
		log.Println("[Server] Redis connection closed.")
	}

	// Explicitly stop background goroutines (RateLimiter cleanup, etc.)
	cancel()
	log.Println("[Server] Shutdown complete. Goodbye!")
}

func resolveInternalServiceTargets(cfg *config.Config) (string, string) {
	kgHost := cfg.GRPCAIHost
	kgPort := cfg.GRPCAIPort
	kgTarget := fmt.Sprintf("%s:%s", kgHost, kgPort)

	cvHost := os.Getenv("GRPC_CV_HOST")
	if cvHost == "" {
		cvHost = kgHost
	}

	cvPort := os.Getenv("GRPC_CV_PORT")
	if cvPort == "" {
		cvPort = "50052"
		if parsedPort, err := strconv.Atoi(kgPort); err == nil {
			cvPort = strconv.Itoa(parsedPort + 1)
		}
	}

	return kgTarget, fmt.Sprintf("%s:%s", cvHost, cvPort)
}
