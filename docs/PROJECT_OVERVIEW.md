# NutriX — Project Overview

> **Document Type**: Developer & Stakeholder Introduction
> **Version**: 1.0
> **Last Updated**: 2026-08-25
> **Evidence**: Full codebase analysis, `ARCHITECTURE.md`, `cmd/server/main.go`, `internal/domain/*.go`

---

## Project Name

**NutriX** — AI-Powered Nutrition Intelligence Platform

---

## One-Sentence Description

NutriX is a backend API that combines computer vision food recognition, knowledge graph-based safety analysis, and personalized nutrition planning to help users track meals, understand food risks, and achieve health goals. [IMPLEMENTED]

---

## Problem Statement

Individual nutritional health management faces three critical challenges:

1. **Food Tracking Burden**: Manual calorie/macro logging is tedious and error-prone, leading to abandonment. [IMPLEMENTED — meal logging API exists]
2. **Health Condition Conflicts**: Users with medical conditions (diabetes, kidney disease, gout) struggle to identify which foods are risky for them. [PARTIAL — KG safety analysis exists but disease context not fully wired]
3. **Personalized Guidance Gap**: Generic nutrition advice doesn't account for individual biometrics, conditions, dietary preferences, or cultural food preferences. [PARTIAL — user profiles + DRI targets exist; KG reasoning is partial]

---

## Target Users

| User Segment | Use Case | Current Readiness |
|---|---|---|
| Health-conscious individuals | Daily meal tracking, calorie counting | ✅ Production-ready |
| Users with medical conditions | Food safety warnings, condition-specific guidance | ⚠️ Partial (KG exists, context wiring incomplete) |
| Fitness enthusiasts | Workout logging, macro balancing | ✅ Production-ready |
| Vietnamese-speaking users | Local food database (VFA dishes) | ✅ Implemented |
| App developers | Integration via REST API | ✅ Production-ready |

[IMPLEMENTED — evidence: `internal/user/delivery/http_handler.go`, `internal/nutrition/delivery/http_handler.go`]

---

## Solution Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        NutriX Backend Architecture                        │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   ┌──────────────┐         ┌──────────────────────┐                      │
│   │   Flutter    │──HTTP──▶│    go_backend        │                      │
│   │   Client    │◀────────│    (Gin + GORM)      │                      │
│   └──────────────┘         └──────────┬───────────┘                      │
│                                      │                                   │
│                          ┌───────────┼───────────┐                       │
│                          │           │           │                       │
│                          ▼           ▼           ▼                       │
│                    ┌──────────┐ ┌──────────┐ ┌──────────┐               │
│                    │PostgreSQL│ │  Redis   │ │  gRPC    │               │
│                    │(primary) │ │ (cache)  │ │ (AI)     │               │
│                    └──────────┘ └──────────┘ └────┬─────┘               │
│                                                   │                      │
│                                    ┌──────────────┴──────────────┐       │
│                                    ▼                              ▼       │
│                              ┌──────────┐                  ┌──────────┐   │
│                              │   ai-kg  │                  │   ai-cv  │   │
│                              │ (Neo4j)  │                  │   (CV)   │   │
│                              └──────────┘                  └──────────┘   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

[IMPLEMENTED — evidence: `docker-compose.prod.yml`, `cmd/server/main.go`, `ARCHITECTURE.md`]

---

## Core Capabilities

### 1. User Authentication & Profile Management [IMPLEMENTED]
- JWT-based authentication (15-min access tokens + rotating refresh tokens)
- User registration/login with biometric data (height, weight, DOB, gender)
- BMR/TDEE calculation using Mifflin-St Jeor equation
- Activity level and goal type configuration
- Dietary preferences (vegan, vegetarian, halal)
- Allergy and medical condition tracking (CSV-based)
- Blind-indexed encrypted search for sensitive health data

**Evidence**: `internal/user/usecase/user_usecase.go`, `pkg/crypto/*.go`

### 2. Food Database & Search [IMPLEMENTED]
- Bundled food databases: VFA (Vietnamese foods), VFA_DISH (Vietnamese dishes), USDA
- Full-text search with trigram matching (pg_trgm extension)
- Spoonacular API fallback for expanded food search
- Search by nutrients (protein, fat, calorie ranges)
- Search by ingredients
- Custom food creation
- Redis-cached KG safety metadata enrichment (24h TTL)

**Evidence**: `internal/nutrition/usecase/nutrition_usecase.go:90-175`, `pkg/database/postgres.go`

### 3. Meal & Water Logging [IMPLEMENTED]
- Log individual food consumption with quantity (grams)
- Meal type categorization (breakfast, lunch, dinner, snack)
- Water intake tracking (ml)
- Profile-based safety gate (blocks conflicting foods unless acknowledged)
- Daily plan aggregation (target vs consumed calories/water)

**Evidence**: `internal/domain/food.go:69-106`, `internal/nutrition/usecase/nutrition_usecase.go:248-290`

### 4. Analytics & Streaks [IMPLEMENTED]
- Daily analytics (calories, water, workouts)
- Weekly analytics (7-day rolling or calendar week)
- Monthly analytics with goal hit tracking
- Streak tracking (consecutive days meeting goals)
- Achievement/gamification system (streak milestones, perfect days)

**Evidence**: `internal/domain/daily_health_snapshot.go`, `internal/domain/streak.go`, `internal/nutrition/service/analytics_aggregation_service.go`

### 5. Computer Vision Food Recognition [PARTIAL]
- Image upload for food photos
- Top-K dish classification candidates
- Mass/volume estimation via Nutrition5k + Depth-Anything models
- Food-to-database resolution
- MinIO object storage integration

**Evidence**: `internal/infrastructure/grpc_ai_client.go`, `docker-compose.prod.yml:147-201`

### 6. Knowledge Graph Safety Analysis [PARTIAL]
- Food safety analysis via Neo4j-backed AI server
- Disease-specific threshold snapshots
- Meal safety validation (APPROVED/WARNING/REJECTED status)
- Violation reporting and fix suggestions
- User feedback submission (corrections, acceptances)

**Evidence**: `internal/domain/nutrition_intelligence_port.go`, `internal/infrastructure/grpc_nutrition_client.go`

### 7. AI-Powered Weekly Planner [EXPERIMENTAL]
- Generate 7-day meal plans
- Profile-based food filtering (allergies, diet, excluded ingredients)
- Meal safety validation via KG
- Plan re-optimization with food swaps
- In-memory plan caching

**Evidence**: `internal/nutrition/usecase/nutrition_usecase.go:generateWeeklyPlan`, `internal/nutrition/usecase/nutrition_usecase.go:reoptimizePlan`

### 8. Workout Tracking [IMPLEMENTED]
- Exercise database from RapidAPI ExerciseDB
- MET-based calorie burn calculation
- Workout logging with duration tracking
- Integration with nutrition analytics (calorie balance)

**Evidence**: `internal/domain/exercise.go`, `internal/workout/usecase/workout_usecase.go`

### 9. Gamification [IMPLEMENTED]
- Achievement system (first goal hit, streak milestones, perfect days)
- Background streak evaluation after meal/water logging
- Achievement unlocking with timestamps

**Evidence**: `internal/domain/achievement.go`, `internal/nutrition/service/gamification_service.go`

---

## Current Scope

### ✅ In Scope (Implemented)

| Feature | Status | Evidence |
|---|---|---|
| User auth (JWT) | ✅ Complete | `internal/user/usecase/user_usecase.go` |
| Food search + DB | ✅ Complete | `internal/nutrition/usecase/nutrition_usecase.go` |
| Meal/water logging | ✅ Complete | `internal/nutrition/usecase/nutrition_usecase.go` |
| Analytics (D/W/M) | ✅ Complete | `internal/nutrition/service/analytics_aggregation_service.go` |
| Streak tracking | ✅ Complete | `internal/nutrition/service/streak_service.go` |
| Workout logging | ✅ Complete | `internal/workout/usecase/workout_usecase.go` |
| Achievements | ✅ Complete | `internal/nutrition/service/gamification_service.go` |
| CV food recognition | ⚠️ Partial | `internal/infrastructure/grpc_ai_client.go` |
| KG safety analysis | ⚠️ Partial | `internal/infrastructure/grpc_nutrition_client.go` |
| Weekly planner | ⚠️ Experimental | `internal/nutrition/usecase/nutrition_usecase.go` |

### ❌ Out of Scope (Not Implemented)

| Feature | Reason |
|---|---|
| Real-time notifications | Not designed yet |
| Social features | Not in current roadmap |
| E-commerce / food delivery | Out of scope |
| Healthcare provider integration | Requires compliance |
| Multi-language UI (beyond VN) | Backend only |
| Mobile push notifications | Client-side feature |
| Offline-first sync | Client-side feature |

---

## Current Maturity

| Dimension | Rating | Notes |
|---|---|---|
| Core Nutrition Tracking | 🟢 Strong | Production-ready for basic use cases |
| Food Database | 🟢 Strong | VFA + USDA + Spoonacular fallback |
| CV Recognition | 🟡 Developing | Service exists, accuracy not validated |
| KG Safety Analysis | 🟡 Developing | Partial context wiring |
| AI Planning | 🟠 Experimental | In-memory cache, limited scaling |
| Observability | 🟢 Strong | Prometheus + Grafana |

---

## Key Dependencies

| Dependency | Version | Purpose | Criticality |
|---|---|---|---|
| Go | 1.25+ | Backend language | 🔴 Critical |
| Gin | latest | HTTP framework | 🔴 Critical |
| GORM | latest | ORM | 🔴 Critical |
| PostgreSQL | 15+ | Primary database | 🔴 Critical |
| Redis | 7+ | Caching, rate limiting | 🟡 Important |
| Neo4j | 5.18.1 | Knowledge graph | 🔴 Critical (for AI features) |
| Python AI Server | latest | KG + CV services | 🔴 Critical (for AI features) |
| MinIO | latest | Object storage | 🟡 Important |
| Cloudflared | latest | Edge tunneling | 🟢 Nice-to-have |
| Docker | latest | Containerization | 🔴 Critical |

---

## Current Limitations

1. **BatchAnalyzeFoods stub**: `BatchAnalyzeFoods()` returns `ErrFeatureUnavailable` — KG batch enrichment fails [IMPLEMENTED — `internal/infrastructure/grpc_nutrition_client.go:108-111`]
2. **Disease context not wired**: Medical conditions CSV not fully propagated to KG calls [PARTIAL — `internal/nutrition/usecase/nutrition_usecase.go:116-117`]
3. **Plan cache in-memory only**: Weekly plans cached in memory map, not Redis/DB [EXPERIMENTAL — `internal/nutrition/usecase/nutrition_usecase.go:39-40`]
4. **No horizontal scaling**: In-memory plan cache won't work across multiple instances
5. **gRPC uses insecure credentials**: Internal network assumed [IMPLEMENTED — `internal/infrastructure/grpc_nutrition_client.go:25`]
6. **Spoonacular API dependency**: External API rate limits may affect food search

---

## One-Sentence Project Definition

NutriX is an AI-powered nutrition tracking backend that combines computer vision food recognition, knowledge graph safety analysis, and personalized meal planning to help users log meals, understand food risks, and achieve health goals.

---

## One-Paragraph Executive Summary

NutriX is a production-ready Go backend API that serves as the central intelligence hub for a nutrition tracking application. It provides comprehensive meal and water logging with calorie/macro tracking, a rich food database spanning Vietnamese cuisine and USDA data, analytics dashboards for daily/weekly/monthly progress, streak and achievement gamification, and workout integration. The platform integrates with Python AI services via gRPC for computer vision food recognition and Neo4j-backed knowledge graph safety analysis. While core nutrition tracking is production-grade, AI features (CV recognition, KG analysis, weekly planning) are in various stages of maturity. The system is containerized with Docker Compose for full-stack deployment including PostgreSQL, Redis, MinIO, and Cloudflare tunneling.

---

## One-Sentence Elevator Pitch

"Track your meals, understand your food's health risks, and get personalized meal plans — all powered by AI that speaks Vietnamese and knows your medical conditions."
