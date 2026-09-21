# NutriX — Codebase Guide

> **Document Type**: Developer Onboarding Guide
> **Version**: 1.0
> **Last Updated**: 2026-08-25
> **Evidence**: Full codebase analysis

---

## Repository Structure

```
nutrix-backend/
├── cmd/                          # Application entry points
│   ├── server/main.go            # Main API server (START HERE)
│   └── seeder/main.go           # Data seeding binary
├── config/
│   └── config.go                 # Environment variable loading
├── internal/                     # Private application code
│   ├── domain/                   # Entities, interfaces, DTOs (READ THIS FIRST)
│   ├── user/                     # User management module
│   ├── nutrition/                # Nutrition tracking module
│   ├── workout/                  # Workout tracking module
│   ├── product/                  # Product data module
│   └── infrastructure/           # External integrations
├── pkg/                          # Shared packages
│   ├── database/                # DB connections, migrations
│   ├── middleware/               # Auth, rate limiting, security
│   ├── crypto/                   # Encryption, hashing
│   ├── spoonacular/              # Spoonacular API client
│   ├── rapidapi/                 # RapidAPI client
│   ├── resilience/               # Circuit breaker
│   ├── rule_engine/              # Expert system (unused)
│   └── orchestrator/             # Background job processing
├── api/proto/                    # gRPC proto definitions (generated)
├── migrations/                    # SQL migrations
├── monitoring/                    # Prometheus + Grafana config
├── docs/                         # This documentation
└── *.json                       # Seed data (VFA, USDA, DRI, MET)
```

---

## Important Directories

### `cmd/server/main.go` — Entry Point

Start here to understand:
- How dependencies are wired
- How the router is configured
- How services are initialized

```go
func main() {
    cfg := config.LoadConfig()           // Load environment
    db, _ := pkgdb.NewPostgresDB(cfg)   // Database connection
    pkgdb.RunMigrations(db)              // Run migrations
    redisClient, _ := pkgdb.NewRedisClient(cfg) // Redis
    
    // Dependency injection
    nutritionRepoInst := nutritionRepo.NewPostgresNutritionRepository(db)
    nutritionUCInst := nutritionUC.NewNutritionUseCase(...)
    
    // Router setup
    r := gin.New()
    r.Use(middleware.RateLimiter(...))
    r.Use(middleware.RequireAuth(...))
    
    // Register handlers
    nutritionDelivery.NewNutritionHandler(protected, nutritionUCInst)
}
```

### `internal/domain/` — Core Domain

The source of truth for all domain models and interfaces.

| File | Contents |
|---|---|
| `user.go` | User, UserPortfolio, RefreshToken entities |
| `food.go` | Food, MealLog, KGMetadata, analytics DTOs |
| `nutrition_intelligence_port.go` | NutritionIntelligencePort (AI/KG interface) |
| `inference_port.go` | InferencePort (CV interface) |
| `dri.go` | DRI (Dietary Reference Intake) model |
| `enums.go` | ActivityLevel, GoalType |
| `rule.go` | RestrictionRule, AggregatedRuleSet |
| `explanation.go` | FoodExplanation, EvidencePath |
| `achievement.go` | Achievement definitions |
| `streak.go` | UserStreak |
| `exercise.go` | Exercise, WorkoutLog, MET activities |

### `internal/nutrition/usecase/nutrition_usecase.go` — Core Business Logic

The largest and most important file (~700+ lines).

**Key sections to read:**

1. **SearchFoods** (line 90-175) — Food search with KG enrichment
2. **LogMeal** (line 248-290) — Meal logging with profile validation
3. **GetDailyPlan** (line 323-409) — Daily aggregation
4. **GenerateWeeklyPlan** (line ~400+) — AI meal planning
5. **EvaluateStreakHook** (line ~600+) — Background streak evaluation

### `internal/infrastructure/` — External Integrations

| File | Purpose |
|---|---|
| `grpc_nutrition_client.go` | gRPC client to ai-kg (Neo4j) |
| `grpc_ai_client.go` | gRPC client to ai-cv (CV) |
| `metrics/middleware.go` | Prometheus metrics |
| `grpc_interceptor.go` | gRPC observability |

---

## Important Modules

### User Module (`internal/user/`)

**Entity**: `User` — authentication + health profile

**Key Files**:
- `delivery/http_handler.go` — Routes: `/api/v1/auth/*`, `/api/v1/users/*`
- `usecase/user_usecase.go` — Auth logic, BMR/TDEE calculation
- `repository/postgres_user.go` — GORM operations with encryption

**Key Functions**:
```go
// User registration with bcrypt
Register(ctx, req *RegisterRequest) (*AuthResponse, error)

// JWT login with refresh token rotation
Login(ctx, req *LoginRequest) (*AuthResponse, error)

// Profile update → recalculates BMR/TDEE
UpdateProfile(ctx, userID, req *UpdateProfileRequest) error

// Calculate personalized DRI-based targets
GetTargets(ctx, userID uuid.UUID) (*UserTargetsResponse, error)
```

### Nutrition Module (`internal/nutrition/`)

**Entity**: `Food`, `MealLog`, `WaterLog`

**Key Files**:
- `delivery/http_handler.go` — All nutrition routes
- `usecase/nutrition_usecase.go` — Core logic
- `repository/postgres_food.go` — Food CRUD
- `service/analytics_aggregation_service.go` — Daily/weekly/monthly stats
- `service/streak_service.go` — Streak evaluation
- `service/gamification_service.go` — Achievement checking

**Key Functions**:
```go
// Search with KG enrichment
SearchFoods(ctx, keyword string) ([]Food, error)

// Core logging with profile validation
LogMeal(ctx, userID, req *LogMealRequest) (*MealLog, error)

// Analytics
GetDailyPlan(ctx, userID, dateStr string) (*DailyPlanResponse, error)
GetWeeklyAnalytics(ctx, userID, days int, isCalendar bool) (*WeeklyAnalyticsResponse, error)

// AI planning
GenerateWeeklyPlan(ctx, userID, req *GenerateWeeklyPlanRequest) (*WeeklyPlanResponseDTO, error)
AnalyzeMeal(ctx, userID, req *AnalyzeMealRequest) (*AnalyzeMealResponse, error)
```

### Workout Module (`internal/workout/`)

**Entity**: `Exercise`, `WorkoutLog`, `MetActivity`

**Key Files**:
- `delivery/http_handler.go` — Routes: `/exercises/*`, `/workouts/log`
- `usecase/workout_usecase.go` — RapidAPI integration

---

## Major Classes/Functions

### Domain Interfaces

```go
// NutritionRepository — data access for nutrition
type NutritionRepository interface {
    GetFoodByID(ctx context.Context, id uuid.UUID) (*Food, error)
    SearchFoods(ctx context.Context, keyword string) ([]Food, error)
    LogMeal(ctx context.Context, log *MealLog) error
    GetDailyLogs(ctx context.Context, userID uuid.UUID, date time.Time) ([]MealLog, error)
    // ... more methods
}

// NutritionUseCase — business logic boundary
type NutritionUseCase interface {
    SearchFoods(ctx context.Context, keyword string) ([]Food, error)
    LogMeal(ctx context.Context, userID uuid.UUID, req *LogMealRequest) (*MealLog, error)
    GetDailyPlan(ctx context.Context, userID uuid.UUID, dateStr string) (*DailyPlanResponse, error)
    // ... more methods
}

// NutritionIntelligencePort — AI/KG abstraction
type NutritionIntelligencePort interface {
    AnalyzeFood(ctx context.Context, userCtx UserNutritionContext, foodID string) (*FoodAnalysisResult, error)
    GetRecommendations(ctx context.Context, userID uuid.UUID, req *GetRecommendationsRequest) (*GetRecommendationsResponse, error)
    AnalyzeMeal(ctx context.Context, req *AnalyzeMealRequest) (*AnalyzeMealResponse, error)
    // ... more methods
}
```

### Key Domain Types

```go
// Food — nutrition data entity
type Food struct {
    ID              uuid.UUID
    Name            string
    CaloriesPer100g float64
    ProteinPer100g  float64
    CarbsPer100g   float64
    FatPer100g      float64
    Micronutrients  datatypes.JSONMap  // All other nutrients (unstructured)
    Source          string             // VFA, VFA_DISH, USDA, Spoonacular
    KGMetadata      *KGMetadata        // gorm:"-" — not persisted
}

// MealLog — food consumption record
type MealLog struct {
    UserID           uuid.UUID
    FoodID           uuid.UUID
    MealType         string    // breakfast, lunch, dinner, snack
    QuantityGrams    float64
    CaloriesConsumed float64
    ProteinConsumed  float64
    FatConsumed      float64
    CarbsConsumed    float64
    ConsumedDate     time.Time
}

// KGMetadata — AI safety signals
type KGMetadata struct {
    IsSafe    bool
    RiskLevel string  // SAFE, LOW, MODERATE, HIGH, SEVERE
    Warnings  []KGWarning
}
```

---

## Configuration Files

### `.env.example` (Not provided, but referenced)

Required environment variables:
```bash
# Database
DB_DSN=postgres://user:pass@host:5432/nutrix

# Redis
REDIS_URL=redis://host:6379
REDIS_PASSWORD=secret

# Auth
JWT_SECRET=your-jwt-secret
JWT_EXPIRATION_HOURS=72

# Security (CRITICAL - panics if missing)
ENCRYPTION_KEYS={"v1":"your-32-byte-key"}
ACTIVE_KEY_VERSION=v1
HMAC_KEY=your-32-byte-hmac-key

# AI Services (CRITICAL - panics if missing)
GRPC_AI_HOST=localhost
GRPC_AI_PORT=50051
GRPC_CV_PORT=50052

# External APIs
SPOONACULAR_API_KEY=your-key
RAPIDAPI_KEY=your-key

# Server
PORT=8080
ALLOWED_ORIGINS=http://localhost:3000
```

### Docker Compose

- `docker-compose.yml` — Development stack
- `docker-compose.prod.yml` — Production stack
- `docker-compose.monitoring.yml` — Prometheus + Grafana
- `docker-compose.edge.yml` — Cloudflared tunnel

---

## Data Models

### User Profile Flow

```
User registers → User created (plaintext password)
               → bcrypt hash stored

User logs in → bcrypt verify
            → JWT access token (15 min)
            → Refresh token (rotating, UUID family)

User updates profile → Recalculate BMR
                     → Recalculate TDEE
                     → Update targets
```

### Food Search Flow

```
Search request → Local Postgres (pg_trgm fuzzy search)
             → Cache check (Redis, key: kg:v2:food:{id}:risk:{profile})
             → Cache miss → gRPC to ai-kg
             → Cache result (24h TTL)
             → Return with KGMetadata
```

### Meal Logging Flow

```
Log request → Validate food exists
           → Check profile conflicts (allergies, conditions)
           → If conflict and !AcknowledgedRisk → 422 MealBlockedError
           → Scale macros by quantity ratio
           → Save to food_logs
           → Trigger streak evaluation (background)
           → Return MealLog
```

---

## Recommended Reading Order

### For New Developers

1. **Start Here**: `cmd/server/main.go` — Understand how everything is wired
2. **Understand Domain**: `internal/domain/user.go` + `internal/domain/food.go` — Core entities
3. **Core Logic**: `internal/nutrition/usecase/nutrition_usecase.go` — Most important file
4. **API Routes**: `internal/nutrition/delivery/http_handler.go` — What endpoints exist
5. **Database**: `internal/nutrition/repository/postgres_food.go` — How data is persisted
6. **AI Integration**: `internal/infrastructure/grpc_nutrition_client.go` — AI/KG interface
7. **Testing**: Look at existing tests for patterns

### For Backend Engineers

1. `internal/domain/` — All interfaces and entities
2. `pkg/database/postgres.go` — Migration system
3. `pkg/middleware/auth.go` — JWT implementation
4. `pkg/crypto/` — Encryption implementation
5. `internal/infrastructure/` — All external integrations

### For AI/ML Engineers

1. `internal/domain/nutrition_intelligence_port.go` — AI interface contract
2. `internal/infrastructure/grpc_nutrition_client.go` — How backend calls AI
3. `internal/domain/nutrition_intelligence_port.go` — DTOs for AI communication

---

## Extension Points

### Adding a New API Endpoint

```go
// 1. Define DTO in domain/
type NewFeatureRequest struct {
    Field1 string `json:"field1"`
}

// 2. Add to usecase interface
type NutritionUseCase interface {
    // existing methods...
    NewFeature(ctx context.Context, userID uuid.UUID, req *NewFeatureRequest) (*Response, error)
}

// 3. Implement in usecase
func (u *nutritionUseCase) NewFeature(ctx context.Context, userID uuid.UUID, req *domain.NewFeatureRequest) (*Response, error) {
    // business logic here
}

// 4. Add route in handler
func NewNutritionHandler(rg *gin.RouterGroup, uc domain.NutritionUseCase) {
    h := &NutritionHandler{uc: uc}
    rg.POST("/new-feature", h.NewFeature)
}

// 5. Implement handler
func (h *NutritionHandler) NewFeature(c *gin.Context) {
    userID := middleware.GetUserID(c)
    var req domain.NewFeatureRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    resp, err := h.uc.NewFeature(c.Request.Context(), userID, &req)
    // handle response
}
```

### Adding a New Database Entity

```go
// 1. Define entity in domain/
type CustomEntity struct {
    ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
    UserID    uuid.UUID `gorm:"index"`
    Data      string
    CreatedAt time.Time
}

// 2. Add to AutoMigrate in pkg/database/postgres.go
if err := db.AutoMigrate(&domain.CustomEntity{}); err != nil {
    return err
}

// 3. Create repository
type customEntityRepository struct {
    db *gorm.DB
}

func (r *customEntityRepository) Create(ctx context.Context, entity *domain.CustomEntity) error {
    return r.db.WithContext(ctx).Create(entity).Error
}
```

### Adding a New External Service

```go
// 1. Create client package
package myservice

type Client struct {
    apiKey string
    baseURL string
}

func NewClient(apiKey string) *Client {
    return &Client{apiKey: apiKey, baseURL: "https://api.example.com"}
}

func (c *Client) FetchData(ctx context.Context, query string) (*Data, error) {
    // HTTP call here
}

// 2. Wire in main.go
myClient := myservice.NewClient(cfg.MyServiceKey)

// 3. Pass to usecase
myUC := NewMyUseCase(myClient, otherDeps)
```

---

## Key Implementation Patterns

### Repository Pattern

```go
// Interface in domain/
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id uuid.UUID) (*User, error)
    // ...
}

// Implementation in repository/
type postgresUserRepository struct {
    db *gorm.DB
}

func NewPostgresUserRepository(db *gorm.DB) domain.UserRepository {
    return &postgresUserRepository{db: db}
}

func (r *postgresUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    var user domain.User
    if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
        return nil, err
    }
    return &user, nil
}
```

### Dependency Injection

```go
// UseCase takes interfaces, not concrete types
type NutritionUseCase struct {
    repo     NutritionRepository
    kgPort   NutritionIntelligencePort  // AI interface
    cvPort   InferencePort               // CV interface
    redis    *redis.Client
}

// Concrete types are created in main.go and passed in
nutritionUC := NewNutritionUseCase(
    nutritionRepoInst,  // implements NutritionRepository
    kgClient,           // implements NutritionIntelligencePort
    cvClient,           // implements InferencePort
    redisClient,
)
```

### Error Handling

```go
// Domain errors are typed
var ErrFoodNotFound = errors.New("food not found")

// Repository returns standard errors
func (r *repo) GetFoodByID(ctx context.Context, id uuid.UUID) (*Food, error) {
    var food Food
    if err := r.db.First(&food, "id = ?", id).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, ErrFoodNotFound
        }
        return nil, err
    }
    return &food, nil
}

// UseCase maps to HTTP
mealLog, err := uc.LogMeal(ctx, userID, req)
if err != nil {
    if errors.Is(err, ErrFoodNotFound) {
        c.JSON(404, gin.H{"error": "food not found"})
        return
    }
    c.JSON(500, gin.H{"error": "internal error"})
}
```

### Background Hooks

```go
// After successful meal log, evaluate streak asynchronously
func (u *nutritionUseCase) LogMeal(...) (*MealLog, error) {
    // ... main logic
    
    // Trigger background hook (non-blocking)
    go u.evaluateStreakHook(context.Background(), userID, date)
    
    return mealLog, nil
}

func (u *nutritionUseCase) evaluateStreakHook(ctx context.Context, userID uuid.UUID, date time.Time) {
    if u.streakService != nil {
        u.streakService.UpdateStreak(ctx, userID, date)
    }
    if u.gamificationService != nil {
        u.gamificationService.CheckAndUnlock(ctx, userID)
    }
}
```
