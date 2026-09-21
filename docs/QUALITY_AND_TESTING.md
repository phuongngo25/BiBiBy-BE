# NutriX — Quality & Testing Documentation

> **Document Type**: Testing & Quality Analysis
> **Version**: 1.0
> **Last Updated**: 2026-08-25
> **Evidence**: `**/*_test.go` glob results

---

## Test Structure Overview

```
internal/
├── infrastructure/
│   ├── grpc_ai_client_test.go
│   ├── grpc_ai_client_phase5_test.go
│   ├── grpc_nutrition_client_e2e_test.go
│   ├── grpc_interceptor_test.go
│   ├── grpc_proto_parity_test.go
│   ├── metrics/
│   │   ├── middleware_test.go
│   │   ├── redis_hook_test.go
│   │   └── middleware_test.go
│   └── delivery/
│       ├── epic1_integration_test.go
│       └── http_handler_validation_test.go
├── nutrition/
│   ├── usecase/
│   │   ├── nutrition_usecase_test.go
│   │   └── ai_food_resolution_test.go
│   ├── service/
│   │   ├── service_test.go
│   │   ├── gamification_service_test.go
│   │   ├── streak_service_test.go
│   │   └── analytics_aggregation_service_test.go
│   └── repository/
│       ├── postgres_snapshot_test.go
│       └── postgres_achievement_repository_test.go
├── user/
│   └── usecase/
│       └── user_usecase_test.go
pkg/
├── middleware/
│   ├── proxy_test.go
│   └── security_test.go
├── database/
│   └── postgres_vfa_dish_test.go
└── crypto/
    └── crypto_test.go
```

---

## Test Coverage Analysis

### Test Files Found: 22

| Category | Files | Coverage Area |
|---|---|---|
| Infrastructure | 7 | gRPC clients, metrics, middleware |
| Nutrition | 9 | Usecase, services, repository |
| User | 1 | User usecase |
| Middleware | 2 | Auth, security |
| Database | 1 | VFA dish seeding |
| Crypto | 1 | Encryption |

---

## Unit Tests

### Unit Test Patterns

#### 1. Service Tests

**Evidence**: `internal/nutrition/service/service_test.go`

```go
// Example: Health calculation service
func TestCalculateBMR(t *testing.T) {
    calc := NewHealthCalculationService()
    
    // Male, 25 years, 70kg, 175cm
    bmr := calc.CalculateBMR(70, 175, dateOfBirth, "male")
    
    // Mifflin-St Jeor: (10 × 70) + (6.25 × 175) − (5 × 25) + 5 = 1693.75
    expected := 1693.75
    assert.InDelta(t, expected, bmr, 1)
}
```

#### 2. Repository Tests

**Evidence**: `internal/nutrition/repository/postgres_snapshot_test.go`

```go
func TestGetOrCreateSnapshot(t *testing.T) {
    // Test creates test database
    // Tests snapshot creation and retrieval
}
```

#### 3. UseCase Tests

**Evidence**: `internal/nutrition/usecase/nutrition_usecase_test.go`

```go
func TestSearchFoods(t *testing.T) {
    // Mock dependencies
    // Test food search with KG enrichment
}
```

---

## Integration Tests

### E2E gRPC Tests

**Evidence**: `internal/infrastructure/grpc_nutrition_client_e2e_test.go`

```go
// Requires running ai-kg service
// Tests actual gRPC communication
func TestNutritionIntelligenceClient_E2E(t *testing.T) {
    if os.Getenv("E2E_TEST") == "" {
        t.Skip("Skipping E2E test")
    }
    
    client, err := NewGrpcNutritionClient("localhost:50051")
    require.NoError(t, err)
    defer client.Close()
    
    // Test HealthCheck
    status, err := client.HealthCheck(context.Background())
    require.NoError(t, err)
    assert.Equal(t, "healthy", status.Status)
}
```

### Epic 1 Integration Test

**Evidence**: `internal/nutrition/delivery/epic1_integration_test.go`

- Tests Epic 1 features (thresholds, feedback, meal validation)
- Requires full stack (Postgres + Redis + AI services)

---

## Contract Tests

### Proto Parity Test

**Evidence**: `internal/infrastructure/grpc/proto_parity_test.go`

```go
// Ensures generated proto stubs match expected structure
func TestProtoContractCompatibility(t *testing.T) {
    // Test that generated types have expected fields
}
```

---

## Test Fixtures

### Database Fixtures

```go
// Common test setup pattern
func setupTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    
    // Run migrations
    err = db.AutoMigrate(&domain.User{}, &domain.Food{})
    require.NoError(t, err)
    
    return db
}
```

### Mock Patterns

```go
// Mock KG client for testing
type mockNutritionClient struct{}

func (m *mockNutritionClient) AnalyzeFood(...) (*FoodAnalysisResult, error) {
    return &FoodAnalysisResult{Safe: true}, nil
}
```

---

## Known Untested Areas

| Area | Evidence | Risk |
|---|---|---|
| Planner (weekly plan) | `nutrition_usecase.go:generateWeeklyPlan` | ⚠️ No dedicated tests |
| CV integration (full) | `grpc_ai_client.go` | ⚠️ Phase 5 tests exist but limited |
| Redis fallback | `pkg/database/redis.go` | ⚠️ No failure mode tests |
| Rate limiter | `pkg/middleware/` | ✅ Basic tests exist |
| Gamification service | `gamification_service.go` | ✅ Tests exist |
| Analytics aggregation | `analytics_aggregation_service.go` | ✅ Tests exist |
| Streak evaluation | `streak_service.go` | ✅ Tests exist |
| User auth | `user_usecase.go` | ✅ Tests exist |

---

## CI/CD Test Integration

### Test Commands

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/nutrition/usecase/...

# Run E2E tests (requires services)
E2E_TEST=1 go test ./internal/infrastructure/... -run E2E
```

### Coverage Requirements

| Package | Target | Current Status |
|---|---|---|
| pkg/middleware | 80%+ | ✅ Likely met |
| internal/nutrition/service | 70%+ | ✅ Likely met |
| internal/user/usecase | 70%+ | ✅ Likely met |
| internal/infrastructure | 50%+ | ⚠️ Partial |

---

## Validation Scripts

### Database Validation

```bash
# Run VFA dish test
go test ./pkg/database/... -run TestVFADishNormalization
```

**Evidence**: `pkg/database/postgres_vfa_dish_test.go`

### Data Integrity Tests

```go
func TestFoodDataIntegrity(t *testing.T) {
    // Verify macronutrients sum correctly
    // Verify micros are stored correctly
}
```

---

## Testing Weaknesses

### 1. No Dedicated Planner Tests

**Evidence**: `nutrition_usecase.go` (no `*_test.go` for planner)

**Risk**: HIGH
- Weekly plan generation has no dedicated tests
- Re-optimization logic untested
- In-memory cache behavior not validated

### 2. CV Integration Partial

**Evidence**: `grpc_ai_client_phase5_test.go`

**Risk**: MEDIUM
- Phase 5 tests exist but may not cover all edge cases
- MinIO integration not tested

### 3. No Performance Tests

**Risk**: MEDIUM
- No load testing
- No latency benchmarks
- No concurrent request tests

### 4. No Chaos Testing

**Risk**: LOW
- Redis failure modes not tested
- Database connection loss not tested
- gRPC timeout scenarios not tested

### 5. AI Service Mocking

**Risk**: MEDIUM
- Tests may depend on real AI services
- No mock AI responses for deterministic testing

---

## Test Recommendations

### P0 — Critical

1. **Add planner unit tests** — Weekly plan generation is experimental and critical
2. **Add AI service mocks** — For deterministic testing
3. **Add rate limiter failure tests** — Redis fallback behavior

### P1 — Important

4. **Add performance benchmarks** — Response time SLAs
5. **Add concurrent request tests** — Thread safety
6. **Expand CV tests** — Mass estimation edge cases

### P2 — Nice to Have

7. **Chaos engineering** — Redis/DB failure injection
8. **Contract tests** — OpenAPI spec validation
9. **Mutation testing** — Test quality enhancement

---

## Files

- Evidence: `internal/nutrition/service/service_test.go`
- Evidence: `internal/nutrition/service/gamification_service_test.go`
- Evidence: `internal/nutrition/service/streak_service_test.go`
- Evidence: `internal/nutrition/service/analytics_aggregation_service_test.go`
- Evidence: `internal/nutrition/usecase/nutrition_usecase_test.go`
- Evidence: `internal/infrastructure/grpc_nutrition_client_e2e_test.go`
- Evidence: `internal/user/usecase/user_usecase_test.go`
- Evidence: `pkg/middleware/security_test.go`
- Evidence: `pkg/crypto/crypto_test.go`
