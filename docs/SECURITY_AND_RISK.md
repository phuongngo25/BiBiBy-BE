# NutriX — Security & Risk Documentation

> **Document Type**: Security Analysis
> **Version**: 1.0
> **Last Updated**: 2026-08-25
> **Evidence**: `pkg/crypto/*.go`, `pkg/middleware/*.go`, `config/config.go`

---

## Security Overview

### Implemented Security Measures

| Measure | Implementation | Evidence |
|---|---|---|
| Password hashing | bcrypt | `internal/user/repository/postgres_user.go` |
| JWT authentication | HS256 + rotating tokens | `pkg/middleware/auth.go` |
| Refresh token rotation | UUID family tracking | `internal/user/usecase/user_usecase.go` |
| Token reuse detection | Family revocation on replay | `internal/user/usecase/user_usecase.go` |
| Sensitive data encryption | AES-256 field encryption | `pkg/crypto/aes_crypto.go` |
| Blind-indexed search | HMAC blind index for allergies | `pkg/crypto/hmac.go` |
| Security headers | HSTS, X-Frame-Options, CSP | `pkg/middleware/security.go` |
| Rate limiting | Redis token bucket | `pkg/middleware/ratelimit.go` |
| CORS configuration | Explicit origin list | `cmd/server/main.go` |
| HTTPS enforcement | RequireHTTPS middleware | `pkg/middleware/security.go` |
| Input validation | Gin binding tags | `internal/domain/*.go` |
| Max body size | 5MB limit | `cmd/server/main.go:199` |

---

## Authentication

### JWT Configuration

| Parameter | Value | Evidence |
|---|---|---|
| Algorithm | HS256 | `pkg/middleware/auth.go` |
| Access token expiry | 15 minutes (default) | `config/config.go:57-62` |
| Refresh token expiry | 72 hours (default) | `config/config.go:57-62` |
| Secret | From env (critical) | `config/config.go:46-50` |

### Token Flow

```
1. Register/Login
   ↓
2. Server generates JWT (15 min) + refresh token (72 hr)
   ↓
3. Client stores both
   ↓
4. Access token expires → client uses refresh token
   ↓
5. Server verifies refresh token, checks for reuse
   ↓
6. If reused → revoke entire token family
   ↓
7. Issue new access + refresh tokens
```

### Token Reuse Detection

```go
// internal/user/usecase/user_usecase.go
// If a refresh token is used twice, the entire family is revoked
// This prevents token theft attacks
```

**Evidence**: `internal/user/usecase/user_usecase.go` (refresh logic)

---

## Password Security

### Hashing

- **Algorithm**: bcrypt
- **Default cost**: Standard bcrypt cost (work factor ~10)
- **Verification**: Constant-time comparison via bcrypt.CompareHashAndPassword

**Evidence**: `internal/user/repository/postgres_user.go`

```go
// Storing password
hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
user.Password = string(hashedPassword)

// Verifying password
err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
```

---

## Sensitive Data Protection

### Encrypted Fields

| Field | Encryption | Index | Evidence |
|---|---|---|---|
| Allergies | AES-256 (blind index only) | HMAC blind index | `AllergiesBidx` column |
| Medical Conditions | AES-256 (blind index only) | HMAC blind index | `MedicalConditionsBidx` column |

### Encryption Implementation

**Evidence**: `pkg/crypto/aes_crypto.go`, `pkg/crypto/hmac.go`

```go
// AES-256 encryption for sensitive fields
type CryptoService struct {
    activeKey    []byte
    keyVersions map[string][]byte
}

// Blind index for search without revealing content
type BlindIndex struct {
    hmacKey []byte
}

func (bi *BlindIndex) Generate(value string) string {
    h := hmac.New(sha256.New, bi.hmacKey)
    h.Write([]byte(value))
    return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
```

### Encryption Configuration

```bash
# Required environment variables (panics if missing)
ENCRYPTION_KEYS={"v1":"your-32-byte-aes-key"}
ACTIVE_KEY_VERSION=v1
HMAC_KEY=your-32-byte-hmac-key
```

**Evidence**: `config/config.go:89-120`

---

## Authorization

### Current Model

| Aspect | Implementation |
|---|---|
| User roles | No RBAC (single role) |
| Resource ownership | User ID from JWT, enforced in queries |
| Admin functions | Not implemented |
| API-level auth | JWT required for protected routes |

### Ownership Enforcement

```go
// Example: User can only update their own logs
func (h *NutritionHandler) UpdateFoodLog(c *gin.Context) {
    userID := middleware.GetUserID(c)  // From JWT
    
    logID, _ := uuid.Parse(c.Param("id"))
    
    // Repository enforces ownership
    updatedLog, err := h.uc.UpdateFoodLog(ctx, userID, logID, quantity)
}
```

---

## Input Validation

### Validation Layers

| Layer | Method | Evidence |
|---|---|---|
| HTTP Handler | Gin binding tags | `internal/domain/*.go` |
| UseCase | Domain validation | `internal/nutrition/usecase/nutrition_usecase.go` |
| Database | Constraint enforcement | PostgreSQL |

### Validation Examples

```go
// Domain validation (food.go)
type LogMealRequest struct {
    FoodID        uuid.UUID `json:"food_id" binding:"required"`
    QuantityGrams float64   `json:"quantity_grams" binding:"required,gt=0,lte=5000"`
    MealType      string    `json:"meal_type" binding:"required"`
    ConsumedDate   string    `json:"consumed_date" binding:"required"`
}

// Email validation
type RegisterRequest struct {
    Email string `json:"email" binding:"required,email"`
}
```

---

## Rate Limiting

### Configuration

| Endpoint Type | Limit | Window |
|---|---|---|
| Public (auth) | 20 requests | 1 minute |
| Protected | 150 requests | 1 minute |

### Implementation

**Evidence**: `pkg/middleware/ratelimit.go`

```go
// Redis-backed token bucket
func RateLimiter(redis *redis.Client, limit int, windowMinutes int) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := fmt.Sprintf("rate_limit:%s", clientIP)
        count, _ := redis.Incr(c, key).Result()
        if count == 1 {
            redis.Expire(c, key, windowMinutes*time.Minute)
        }
        if count > limit {
            c.AbortWithStatus(429)
            return
        }
        c.Next()
    }
}
```

### Fallback Behavior

If Redis is unavailable:
- Rate limiting is bypassed (fail-open)
- This is documented as degraded mode

---

## Security Headers

### Implemented Headers

**Evidence**: `pkg/middleware/security.go`

| Header | Value | Purpose |
|---|---|---|
| X-Frame-Options | DENY | Prevent clickjacking |
| X-Content-Type-Options | nosniff | Prevent MIME sniffing |
| X-XSS-Protection | 1; mode=block | XSS filter (legacy browsers) |
| Strict-Transport-Security | max-age=31536000 | Force HTTPS |
| Content-Security-Policy | (API mode: none) | Disabled for API |

---

## Risk Analysis

### Risk Matrix

| Risk | Impact | Likelihood | Current Mitigation | Remaining Gap |
|---|---|---|---|---|
| JWT secret weak/default | 🔴 Critical | 🟡 Medium | Warning log if dev secret | No enforcement in prod |
| AI service impersonation | 🔴 Critical | 🟡 Low | Internal network assumed | No mTLS |
| SQL injection | 🟠 High | 🟢 Low | GORM parameterized | N/A |
| Redis data leakage | 🟠 High | 🟢 Low | Password required | No encryption at rest |
| Sensitive field exposure | 🟠 High | 🟡 Medium | Encryption + blind index | Backup encryption |
| Rate limit bypass | 🟡 Medium | 🟡 Medium | Redis-based | Fail-open is risk |
| AI hallucination | 🟡 Medium | 🟡 Medium | Human-in-the-loop | Limited explainability |
| Refresh token theft | 🟡 Medium | 🟡 Low | Reuse detection + revocation | No token binding |
| Password brute force | 🟡 Medium | 🟡 Medium | Rate limiting | No CAPTCHA |
| CORS misconfiguration | 🟡 Medium | 🟢 Low | Explicit origin list | Must be maintained |

---

## External Service Risks

### Spoonacular API

| Risk | Impact | Mitigation |
|---|---|---|
| API key exposure | 🔴 High | Server-side only |
| Rate limit exhaustion | 🟡 Medium | Fallback to local DB |
| Service downtime | 🟡 Medium | Graceful degradation |

### RapidAPI ExerciseDB

| Risk | Impact | Mitigation |
|---|---|---|
| API key exposure | 🔴 High | Server-side only |
| Rate limit exhaustion | 🟡 Medium | Cache exercise catalog |
| Service downtime | 🟡 Medium | Cached exercises available |

### AI Services (gRPC)

| Risk | Impact | Mitigation |
|---|---|---|
| No authentication | 🔴 Critical | Internal network only |
| No encryption | 🔴 Critical | Network isolation |
| AI hallucination | 🟡 Medium | Human-in-the-loop |
| Data leakage to AI | 🟠 High | No PII sent to AI |

---

## Data Protection

### Data Classification

| Data Type | Sensitivity | Protection |
|---|---|---|
| User password | 🔴 High | bcrypt hash |
| JWT tokens | 🟠 Medium | Server secret only |
| Refresh tokens | 🟠 Medium | SHA-256 hash in DB |
| Allergies | 🔴 High | Blind index |
| Medical conditions | 🔴 High | Blind index |
| Health metrics | 🟡 Medium | JWT auth only |
| Meal logs | 🟡 Medium | JWT auth only |
| Food database | 🟢 Low | Public data |

### GDPR Considerations

- **Right to deletion**: Cascade deletes via FK constraints
- **Data portability**: JSON export possible (manual)
- **Consent**: Not tracked in current implementation
- **Blind indexing**: Ready for encrypted search

---

## Operational Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Database outage | 🔴 Critical | Replication needed |
| Redis data loss | 🟡 Medium | AOF persistence enabled |
| AI service failure | 🟡 Medium | Graceful degradation |
| Deployment failure | 🟡 Medium | Blue-green not implemented |
| Secret rotation | 🟡 Medium | Key versioning in place |
| Backup failure | 🔴 Critical | Backup not configured in compose |

---

## Security Gaps

### High Priority Gaps

1. **No mTLS for gRPC**
   - AI services use insecure credentials
   - Network isolation is only protection

2. **No backup configuration**
   - PostgreSQL backup not in docker-compose
   - No disaster recovery plan

3. **No API key for AI services**
   - Internal network trust model only
   - Vulnerable to internal threats

### Medium Priority Gaps

4. **No rate limit on /metrics endpoint**
   - Potential for Prometheus scraping abuse

5. **No CSP for SPA clients**
   - API mode disables CSP

6. **No token binding**
   - Refresh tokens not bound to device/session

---

## Compliance Readiness

| Standard | Status | Evidence |
|---|---|---|
| GDPR | ⚠️ Partial | No consent tracking, basic deletion |
| CCPA | ⚠️ Partial | No data sale provisions |
| HIPAA | ❌ Not compliant | No BAA, no encryption at rest |
| SOC 2 | ❌ Not compliant | No audit logging |

**Note**: Current implementation is NOT HIPAA compliant. Medical conditions are stored in plaintext (comma-separated) with only blind index encryption.

---

## Security Recommendations

### P0 — Critical

1. **Implement backup strategy**
   - Configure PostgreSQL backup
   - Test restore procedures

2. **Add mTLS for gRPC**
   - Use TLS credentials for AI service communication
   - Certificate rotation

3. **Add secret scanning to CI**
   - Detect committed secrets

### P1 — Important

4. **Implement API rate limiting per user**
   - Currently IP-based only

5. **Add audit logging**
   - Log sensitive operations

6. **Add CSP headers for web clients**

### P2 — Nice to Have

7. **Token binding to device**
8. **Password strength enforcement**
9. **Multi-factor authentication**
