# NutriX `go_backend` — Architecture Reference

> Written for a future engineer or AI with no prior memory of this system.
> Goal of this document: explain how `go_backend` is structured, how it talks to
> the AI Knowledge Graph (`ai-kg`), and exactly how it models health/diet data —
> because the next major work item is **expanding the Knowledge Graph** (diseases,
> allergens, nutrient threshold rules).

---

## 1. Overview

`go_backend` is the **central REST API** for the NutriX nutrition app. It is the
single entry point for the Flutter client (`Nutrix` repo) and the orchestrator
that fans requests out to the two Python AI services in the `AI_server` repo:

- **`ai-kg`** — Knowledge Graph on Neo4j (food safety, disease rules, thresholds, recommendations).
- **`ai-cv`** — Computer-vision food recognition (image → food label + estimated mass).

```
Flutter (Nutrix)  ──HTTP/JSON──►  go_backend (Gin)  ──gRPC──►  ai-kg  (Neo4j)
                                       │            ──gRPC──►  ai-cv  (CV models)
                                       ├── Postgres (GORM)  — source of truth for users, foods, logs
                                       └── Redis            — rate limiting + KG enrichment cache
```

The architectural contract is stated in `internal/domain/nutrition_intelligence_port.go`:

> *Go Backend (Source of Truth for Facts) builds UserContext from its own DB,
> then passes it to the AI Server (Source of Truth for Intelligence) via this port.*
> *The AI Server NEVER queries user data directly.*
> *Flow: Flutter → Go Backend (JWT verify) → gRPC → AI Server → Neo4j*

### Tech stack

| Concern | Technology |
|---|---|
| HTTP framework | Gin (`github.com/gin-gonic/gin`) |
| ORM / DB | GORM + PostgreSQL 15 (extensions: `pg_trgm`, `unaccent`) |
| Cache / rate limit | Redis 7 (`github.com/redis/go-redis/v9`) |
| AI transport | gRPC (`google.golang.org/grpc`), insecure creds (internal network) |
| Auth | JWT (HS256, 15-min access token) + rotating refresh tokens |
| Observability | Prometheus middleware + interceptors, Grafana dashboards |
| External food data | Spoonacular, OpenFoodFacts proxy, RapidAPI ExerciseDB |
| Edge | Cloudflared tunnel, nginx |

Config is loaded in `config/config.go` via `LoadConfig()`. Several vars are
**fatal if missing** (`panic`): `ENCRYPTION_KEYS`, `ACTIVE_KEY_VERSION`,
`HMAC_KEY`, `GRPC_AI_HOST`, `GRPC_AI_PORT`.

---

## 2. Directory map

The codebase follows **Clean Architecture per feature**: each feature has
`domain` (entities + interfaces, shared), `usecase` (business logic),
`delivery` (HTTP handlers), `repository` (Postgres), and sometimes `service`
(stateless helpers) and `seeder`.

```
cmd/
  server/main.go        # API entrypoint: DI wiring, migrations, seeders, gin router, graceful shutdown
  seeder/main.go        # Standalone seeder binary (used by the go-seeder docker service)
  audit_sprint_11_5/    # one-off audit tools
  audit_sprint_11_6/

config/config.go        # Env loader -> Config struct

internal/
  domain/               # SHARED entities + repository/usecase/port interfaces (no logic deps)
    user.go             #   User, UserPortfolio, RefreshToken, profile DTOs + interfaces
    food.go             #   Food, KGMetadata, MealLog, planner DTOs, NutritionRepository/UseCase ifaces
    nutrition_intelligence_port.go  # NutritionIntelligencePort (the KG gRPC abstraction) + all KG DTOs
    inference_port.go   #   InferencePort (the CV gRPC abstraction)
    dri.go              #   DRI (Dietary Reference Intake) GORM model + DRIRepository
    enums.go            #   ActivityLevel, GoalType enums
    rule.go             #   RestrictionRule / AggregatedRuleSet / RuleType / MacroType (expert-system types)
    explanation.go      #   Explanation, FoodExplanation, EvidencePath, EvidenceNode
    daily_health_snapshot.go  # DailyHealthSnapshot (frozen daily targets)
    exercise.go, water.go, streak.go, achievement*.go, timeline.go, errors.go

  nutrition/
    delivery/http_handler.go         # /nutrition/*, /planner/* routes
    delivery/gamification_handler.go # /gamification/* routes
    usecase/nutrition_usecase.go     # **CORE FILE** — KG enrichment, planner, meal analysis (~1900 LOC)
    usecase/gamification_usecase.go
    repository/postgres_food.go, postgres_dri.go, postgres_streak_repository.go, postgres_achievement_repository.go
    service/   analytics_aggregation_service.go, gamification_service.go, streak_service.go,
               health_calculation_service.go (BMR/TDEE), goal_strategy.go (calorie/water targets)
    seeder/seeder.go                 # USDA / VFA food + dish + DRI seeding

  user/
    delivery/http_handler.go         # /api/v1/auth/* (public) + /users/* (protected)
    usecase/user_usecase.go          # register/login/refresh, profile, portfolio, GetTargets (DRI math)
    repository/postgres_user.go, postgres_user_portfolio.go

  workout/
    delivery/http_handler.go         # /exercises*, /workouts/log
    usecase/workout_usecase.go, local_catalog.go
    repository/postgres_repository.go
    seeder/met_seeder.go             # MET activity seeding

  product/
    delivery/off_proxy_handler.go    # OpenFoodFacts same-origin reverse proxy

  infrastructure/
    grpc_nutrition_client.go         # **KG gRPC client** (implements NutritionIntelligencePort)
    grpc_ai_client.go                # CV gRPC client (implements InferencePort)
    grpc/pb/intelligencepb/          # generated stubs for NutritionIntelligenceService (ACTIVE contract)
    grpc/pb/commonpb/                # generated RequestMeta, RiskLevel, EvidencePath
    grpc/pb/inferencev1/, nutrix/inference/v1/  # CV stubs
    metrics/                         # Prometheus middleware, gRPC + Redis interceptors

pkg/
  database/postgres.go   # NewPostgresDB, RunMigrations (AutoMigrate + SQL files), SeedDummyFoods
  database/redis.go      # NewRedisClient
  middleware/            # auth (RequireAuth/GetUserID), security headers, rate limiter
  crypto/                # AES-256 field encryption, HMAC blind-index, token hashing
  spoonacular/, rapidapi/, orchestrator/, resilience/ (circuit breaker), rule_engine/

api/proto/inference.proto            # CV proto (the KG proto lives in the AI_server repo)
migrations/*.sql                     # explicit SQL migrations (001..005)
*.json                               # seed data (see §7)
docker-compose.prod.yml              # full prod stack
```

---

## 3. Feature modules

### Auth / User / Profile (`internal/user`)

- **Public routes** (`NewUserHandler`, rate-limited 20 req/min):
  `POST /api/v1/auth/register`, `/login`, `/refresh`.
- **Protected routes** (`RegisterProfileRoutes`, under `/api/v1`, JWT required):
  `PUT /users/profile`, `GET /users/profile`, `GET /users/profile/targets`,
  `GET /users/portfolio`, `PUT /users/portfolio`.
- **Usecase** (`user_usecase.go`): bcrypt password hashing; password strength check;
  JWT (15-min) + refresh-token rotation with **reuse detection** (revokes the whole
  token family on replay); `UpdateProfile` recomputes BMR/TDEE via
  `HealthCalculationService`; `GetTargets` computes personalized macro + micronutrient
  targets from the **DRI** table and aggregates today's intake from meal logs.
- **Persistence**: `users`, `user_portfolios`, `refresh_tokens` tables.
  Allergies and medical conditions are stored with a **blind index** column
  (`AllergiesBidx`, `MedicalConditionsBidx`) for encrypted-search support.

### Nutrition (`internal/nutrition`) — the largest module

- **Routes** (`NewNutritionHandler` under `/api/v1`):
  - Foods: `GET /nutrition/foods/search`, `POST /nutrition/foods`,
    `POST /nutrition/foods/upload-image`, `POST /nutrition/foods/estimate` (CV),
    `GET /nutrition/search-spoonacular`, `/search-by-nutrients`, `/search-by-ingredients`.
  - Logging: `POST /nutrition/log-meal`, `POST /nutrition/log-water`,
    `PUT /nutrition/logs/:id`, `GET /nutrition/daily-plan`.
  - Analytics: `GET /nutrition/analytics/{day,weekly,monthly,streak}`.
  - **KG-backed**: `POST /nutrition/recommendations`, `GET /nutrition/thresholds`,
    `POST /nutrition/feedback/{correction,acceptance,viewed}`,
    `POST /nutrition/meal/validate`.
  - **Planner**: `POST /planner/weekly-plan`, `POST /planner/reoptimize`,
    `POST /planner/explain`.
- **Usecase responsibilities** (`nutrition_usecase.go`): food search with KG
  enrichment (§4), meal/water logging, daily/weekly/monthly analytics,
  CV image → food resolution, weekly plan generation + swap re-optimization,
  meal safety validation, threshold/feedback proxying to KG.
- **Domain models**: `Food`, `MealLog` (→ table `food_logs`), `WaterLog`,
  `DailyHealthSnapshot`, `KGMetadata`.
- **Persistence**: `foods`, `food_logs`, `water_logs`, `daily_health_snapshots`,
  `user_streaks`, `user_achievements`, `dris`.

### Workout (`internal/workout`)

- Routes: `GET /exercises`, `/exercises/heatmap`, `/exercises/asset`,
  `/exercises/:id`, `POST /workouts/log`.
- Pulls exercise data from RapidAPI ExerciseDB; computes burned calories from MET
  activities (`met_activities.json`). Feeds burned-calorie totals into nutrition analytics.

### Product (`internal/product`)

- `off_proxy_handler.go` — same-origin reverse proxy to OpenFoodFacts with the
  User-Agent string OFF requires (`NutriX/1.0 (https://bibiby.space)`).

### Gamification (`internal/nutrition`, separate handler)

- `GET /gamification/achievements`. Streak + achievement evaluation runs as a
  **background hook** (`evaluateStreakHook`) after meal/water logging.

---

## 4. KG integration (the important part)

### 4.1 The gRPC abstraction

The KG is reached through the **port interface** `domain.NutritionIntelligencePort`
(`internal/domain/nutrition_intelligence_port.go`). Its concrete implementation is
`grpcNutritionClient` in `internal/infrastructure/grpc_nutrition_client.go`.

It is constructed in `cmd/server/main.go`:

```go
kgTarget, cvTarget := resolveInternalServiceTargets(cfg)   // KG = GRPC_AI_HOST:GRPC_AI_PORT (default 50051)
kgClient, kgErr := infrastructure.NewGrpcNutritionClient(kgTarget)
...
nutritionUCInst := nutritionUC.NewNutritionUseCase(
    nutritionRepoInst, streakRepoInst, achievementRepoInst, spoonClient,
    workoutRepoInst, uRepo, cvClient, kgClient, redisClient, userPortfolioRepoInst)
```

> Note: CV defaults to `GRPC_AI_PORT + 1` (i.e. `50052`) unless `GRPC_CV_PORT` is set.

### 4.2 The proto contract

The **active** generated stubs are in
`internal/infrastructure/grpc/pb/intelligencepb/`
(package `intelligencepb`). The **source `.proto`** lives in the AI_server repo at
`AI_server/api/proto/v1/nutrition_intelligence.proto`
(service `nutrix.intelligence.v1.NutritionIntelligenceService`).

RPCs defined in the proto:
`HealthCheck`, `AnalyzeFood`, `GetThresholdSnapshot`, `SubmitFoodCorrection`,
`GetFeedbackAnalytics`, `GetNutritionGap`, `GetRecommendations`, `AnalyzeMeal`.

Which RPCs `go_backend` actually **calls** (in `grpc_nutrition_client.go`):
`HealthCheck`, `AnalyzeFood`, `GetRecommendations`, `AnalyzeMeal`,
`GetThresholdSnapshot`, `SubmitFoodCorrection`.
`BatchAnalyzeFoods` and `ExplainFood` are **stubbed out** — they return
`domain.ErrFeatureUnavailable` (no RPC exists for them in the v1 contract).

### 4.3 Redis-cached KG enrichment (food search)

`SearchFoods` (`nutrition_usecase.go`) enriches each food with KG safety metadata:

1. Search local Postgres; fall back to Spoonacular if empty.
2. For each food, build cache key `kg:v2:food:<foodID>:risk:<profileHash>`
   (profileHash is currently the literal `"default_profile"`; diseaseIDs is an
   empty slice — health context is **not yet wired** into this path).
3. On Redis hit → unmarshal cached `domain.KGMetadata`. On miss → collect IDs.
4. For misses, call `kgPort.BatchAnalyzeFoods(missingFoodIDs, diseaseIDs)`.
   **Because `BatchAnalyzeFoods` is a stub returning `ErrFeatureUnavailable`, this
   path currently always fails** and each missing food is stamped with a fallback
   `KGMetadata{IsSafe:false, RiskLevel:"UNKNOWN", Warnings:[{Code:"SRE_UNAVAILABLE"}]}`.
5. On a (hypothetical) success, results are cached with **TTL = 24h**
   (`u.redis.Set(ctx, cacheKey, json, 24*time.Hour)`).

`KGMetadata` (`food.go`) is `gorm:"-"` — it is never persisted, only attached to
the JSON response.

### 4.4 Food safety analysis & the planner

The KG safety gate is `AnalyzeMeal`. `grpcNutritionClient.AnalyzeMeal` sends a
`CandidateMeal{meal_id, food_ids, meal_type, ingredients, categories, protein_sources}`
and consumes `status` (APPROVED/WARNING/REJECTED), `score`, `violations`, `fixes`.

> Important: the gRPC `AnalyzeMealRequest` carries **only the candidate meal — no
> user health profile, no allergies, no nutrient totals.** Health-aware gating on
> the Go side is done **locally** (see §4.5 and the Report).

**`GenerateWeeklyPlan`** (planner):
1. Load up to 100 candidate foods from DB (or use client-supplied IDs).
2. Pre-filter locally by user profile + portfolio (`plannerFoodAllowedByProfile`):
   drops foods matching the user's allergies; enforces vegan/vegetarian/halal diet;
   drops portfolio `ExcludedIngredients`.
3. For each survivor, call `kgPort.AnalyzeMeal`.
4. **Fail-closed on hard `REJECTED`** — but for DB-generated candidates, a
   KG-`UNKNOWN`/unavailable result is treated as a *coverage gap* and accepted as
   `WARNING` (`plannerKGUnavailable`). If everything is rejected →
   `ErrAllCandidatesRejected` (HTTP 422).
5. Expand the accepted meals across 7 days × meal slots; cache the plan in an
   **in-memory** `planCache map[planID]*WeeklyPlanResponseDTO` (not Redis, not DB).

**`ReoptimizePlan`** (swap logic): for a `swap_meal` adjustment, pull a fresh pool
of 40 foods, skip the current food, re-apply profile filtering, then call
`AnalyzeMeal` per candidate (bounded to 20 KG calls) and take the first
safe/accepted one. Same fail-closed-on-REJECTED rule.

**`AnalyzeMeal` usecase** (for `/nutrition/meal/validate`): calls KG, then *also*
runs a **local catalog analysis** (`analyzeMealFromCatalog`) that uses the user's
profile to add diet/allergy violations and enriches the dish with an ingredient
breakdown. If the KG is unavailable, the local result is returned as a fallback.

---

## 5. Data models (health / diet)

### 5.1 `User` (`internal/domain/user.go`, table `users`)

| Field | Column / type | Notes |
|---|---|---|
| `HeightCm`, `WeightKg` | float64 | biometrics |
| `DOB` | date | nullable |
| `Gender` | string | free text ("male"/"female"/...) |
| `ActivityLevel` | `ActivityLevel` enum | see below |
| `BMR`, `TDEE` | float64 | computed |
| `GoalType` | `GoalType` enum | |
| `WeeklyCalorieBudget` | float64 | |
| **`DietaryPreference`** | string | free text; planner checks `"vegan"/"vegetarian"/"halal"` |
| **`Allergies`** | `text` | **comma-separated string**, not an enum/table |
| `AllergiesBidx` | string, indexed | blind index for encrypted search |
| **`MedicalConditions`** | `text` | **comma-separated string** of disease IDs |
| `MedicalConditionsBidx` | string, indexed | blind index |

> **There is no diseases table and no allergens enum.** "Health conditions" and
> "allergies" are stored as **free-text CSV** on the `users` row. When sending
> disease context to the KG, the usecase does
> `strings.Split(user.MedicalConditions, ",")` (see `ExplainFood`,
> `GetThresholdSnapshot`).

Enums (`internal/domain/enums.go`):
- `ActivityLevel`: `sedentary`, `low_active`, `active`, `very_active`.
- `GoalType`: `lose_weight`, `maintain`, `gain_weight`.

### 5.2 `UserPortfolio` (table `user_portfolios`, 1:1 with user)

JSONB personalization that doesn't belong on the auth row:
`PreferredCuisines`, `DislikedIngredients`, `ExcludedIngredients` (JSON string
arrays), `MealSchedule` (JSONB), `DailyWaterTargetML`, `CalorieTargetOverride`,
`MacroSplitOverride` (JSONB), `Notes`.

### 5.3 `Food` (`internal/domain/food.go`, table `foods`)

| Field | Column | Notes |
|---|---|---|
| `ID` | uuid PK | |
| `SpoonacularID` | unique | nullable |
| `Code` | unique | e.g. `VFA-...`, `DISH-...`, `USDA-...`, `SPOON-...` |
| `Name`, `NameVi`, `NameEn`, `Category` | | |
| `Source` | default `'VFA'` | `VFA` / `VFA_DISH` / `USDA` / `Spoonacular` / `custom` |
| `CaloriesPer100g`, `ProteinPer100g`, `CarbsPer100g`, `FatPer100g` | float64 | **the only first-class macros** |
| `ServingSize` | string | e.g. `"650g"` |
| **`Micronutrients`** | `jsonb` (`datatypes.JSONMap`) | **everything else** lives here as `"<name>": "<value> <unit>"` |
| `IsVegan/IsVegetarian/IsGlutenFree/IsDairyFree` | bool | |
| `KGMetadata` | `gorm:"-"` | response-only KG safety signals |

> **There are NO dedicated nutrient columns** for sodium, potassium, phosphorus,
> purine, glycemic index, saturated fat, etc. Only the four macros are columns.
> All micronutrients are unstructured key/value strings inside the
> `micronutrients` JSONB blob (e.g. `"Sodium": "120.000000 mg"`), keyed by the
> source dataset's nutrient name. Threshold/limit logic against these requires
> string parsing (`parseMicroVal` in `user_usecase.go`).

### 5.4 `DRI` (`internal/domain/dri.go`, table `dris`)

Dietary Reference Intakes per life-stage/age-range, seeded from `DRIs.json`.
Stored as JSONB columns `rda_ai`, `ear`, `ul`, `amdr`. Structured fields exist for
**many nutrients** here (calcium, iron, sodium, potassium, phosphorus, zinc,
vitamins A–K/B-complex, macros, fiber, water, AMDR percent ranges) — this is the
**richest structured nutrient model in the backend** and is the natural place to
attach per-nutrient limits. `DRIRepository.GetByDemographic` is used by
`GetTargets` to build per-user micronutrient targets.

### 5.5 Rules / thresholds domain (currently mostly latent)

- `internal/domain/rule.go`: `RestrictionRule` (hard/soft, `EXCLUDE_INGREDIENT` /
  `EXCLUDE_TAG` / `LIMIT_MACRO`, `MaxLimit`), `AggregatedRuleSet`. These types exist
  for an in-memory expert system (`pkg/rule_engine/`) but are not on the main
  request path today.
- KG thresholds DTOs (`nutrition_intelligence_port.go`): `ThresholdSnapshot` +
  `NutrientThresholdSnapshot{NutrientID, WarningMg, CriticalMg}`. Fetched from the
  KG via `GetThresholdSnapshot(diseaseIDs)`; **the only place a "warning vs
  critical per-nutrient threshold" concept exists**, and it is owned by `ai-kg`.

---

## 6. How to run

### Dev (local)

1. Provide a `.env` (see `.env.example`). Required-or-panic:
   `ENCRYPTION_KEYS` (JSON map, active key must be 32 bytes), `ACTIVE_KEY_VERSION`,
   `HMAC_KEY` (32 bytes), `GRPC_AI_HOST`, `GRPC_AI_PORT`. Common others:
   `DB_DSN`, `REDIS_URL`, `REDIS_PASSWORD`, `JWT_SECRET`, `SPOONACULAR_API_KEY`,
   `RAPIDAPI_KEY`, `ALLOWED_ORIGINS`, `PORT` (default 8080).
2. `go run ./cmd/server` — on boot it runs migrations, seeds bundled foods + DRIs +
   MET activities, connects Redis (degrades gracefully if down), dials the KG/CV
   gRPC servers (logs and continues if unavailable), then serves on `:8080`.
3. Seed-only: `go run ./cmd/seeder`.

### Docker (prod) — `docker-compose.prod.yml`, project `nutrix-prod`

| Service | Image / build | Port(s) | Role |
|---|---|---|---|
| `postgres` | postgres:15-alpine | 5432 | main app DB |
| `redis` | redis:7-alpine | 6379 | cache / rate limit (password, AOF, 256mb LRU) |
| `minio` | minio/minio | 9000/9001 | object store (CV images) |
| `ai-postgres` | postgres:17 | 5433→5432 | AI services' own DB |
| `neo4j` | neo4j:5.18.1 | 7474/7687 | **Knowledge Graph store** |
| `ai-kg` | build `../AI_server` | 50051 | KG gRPC server (Neo4j-backed) |
| `ai-cv` | build `../AI_server/Computer_Vision` | 50052, 8081 | CV gRPC (+ NVIDIA GPU reservation, CPU fallback) |
| `go-seeder` | build `.` → `/app/seeder` | — | runs once, seeds DB, exits |
| `go-backend` | build `.` | 8080 | this API |
| `cloudflared` | cloudflare/cloudflared | — | edge tunnel |

Startup ordering: `go-backend` depends on `postgres`+`redis` healthy,
`go-seeder` completed, and `ai-kg`+`ai-cv` healthy. Inside the compose network the
backend reaches the KG at `GRPC_AI_HOST=ai-kg`, `GRPC_AI_PORT=50051` and CV at
`ai-cv:50052`. Env for both Go services is inlined in the compose file.

Other compose files: `docker-compose.yml` (base/dev), `docker-compose.edge.yml`,
`docker-compose.monitoring.yml` (Prometheus + Grafana), `docker-compose.spark.yml`.

### Migrations

`pkg/database/RunMigrations(db)` runs on every boot (idempotent): enables
`pg_trgm` + `unaccent`, drops legacy `exercise_logs`, runs **GORM AutoMigrate**
for `User, UserPortfolio, Food, MealLog, Exercise, MetActivity, WorkoutLog, DRI,
RefreshToken, WaterLog`, then applies the explicit SQL files in `migrations/` in
order.

---

## 7. Migrations & seed data

### `migrations/` (explicit SQL, applied after AutoMigrate)

- `001_add_timezone_to_users.sql` — adds `timezone` to users.
- `002_create_daily_health_snapshots.sql` — `daily_health_snapshots` (frozen daily
  targets; unique on `(user_id, snapshot_date)`).
- `003_create_user_streaks.sql` — streak tracking.
- `004_create_user_achievements.sql` — gamification unlocks.
- `005_normalize_pho_thin_nutrition.sql` — data fix for a specific dish.

### Seed files (root-level `*.json`, loaded by `internal/nutrition/seeder/seeder.go`)

| File | Seeds → | Notes |
|---|---|---|
| `vfa_food_db.json` | `foods` (source `VFA`) | Vietnamese Food Composition ingredients; macros mapped, rest → `micronutrients` JSONB |
| `vfa_dishes_db.json` | `foods` (source `VFA_DISH`, category "Prepared Dish") | composite VN dishes (pho, com tam…); per-serving normalization for known codes |
| `usda_core_foods.json` | `foods` (source `USDA`) | USDA FDC foods; kcal prioritized over kJ |
| `DRIs.json` | `dris` | Dietary Reference Intakes per life stage (RDA/AI, EAR, UL, AMDR) |
| `met_activities.json` | `met_activities` (workout seeder) | MET values for calorie burn |
| `tags.json` | — | **currently empty** (`{"count":0,...,"results":[]}`) |

Macro parsing in the seeder is keyword-based on the source dataset's nutrient
names (`energy`→calories, `protein`, `lipid`/`fat`, `carbohydrate`); everything
else is dumped verbatim into `micronutrients`.

> **No disease, allergen, or nutrient-threshold-rule data is seeded by
> `go_backend`.** Those concepts live exclusively in `ai-kg`'s Neo4j graph. The
> Go backend only seeds foods + DRIs. This is the key constraint for the planned
> KG expansion (see the accompanying report).

---

## 8. Notes for the KG expansion

- Disease/allergen/threshold knowledge is owned by `ai-kg` (Neo4j). `go_backend`
  passes disease IDs *as strings* and trusts the KG for safety verdicts.
- The current gRPC contract does **not** transmit user allergies, dietary
  preference, or current nutrient totals to the KG for `AnalyzeMeal`/`AnalyzeFood`.
  Expanding the KG to reason over these will require **proto changes**
  (`AI_server/api/proto/v1/nutrition_intelligence.proto`) + regenerated
  `intelligencepb` stubs + populating the new request fields in
  `grpc_nutrition_client.go` from `domain.UserNutritionContext`.
- `UserNutritionContext` + `UserDisease{ID,Name,Severity}` already model the
  richer per-user health context (with severity) but are only partially populated
  today (e.g. `ExplainFood` fills diseases from CSV with empty severity).
- For per-nutrient limits on the Go side, the `dris` table (UL = tolerable upper
  limits) is the most structured existing source; per-food nutrient values needed
  for threshold checks are unstructured strings in `foods.micronutrients`.
