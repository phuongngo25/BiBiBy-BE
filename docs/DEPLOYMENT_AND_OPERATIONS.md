# NutriX — Deployment & Operations Documentation

> **Document Type**: Operations Guide
> **Version**: 1.0
> **Last Updated**: 2026-08-25
> **Evidence**: `docker-compose.prod.yml`, `cmd/server/main.go`, `Dockerfile`

---

## Deployment Architecture

### Production Stack

```mermaid
flowchart TB
    subgraph Edge
        Cloudflare["Cloudflare Tunnel"]
        User["End Users"]
    end

    subgraph Production["Docker Network: nutrix"]
        subgraph Backend
            Seeder["go-seeder"]
            Backend["go-backend"]
        end
        subgraph Data
            Postgres["PostgreSQL :5432"]
            Redis["Redis :6379"]
            MinIO["MinIO :9000"]
        end
        subgraph AI
            AI_PG["AI Postgres :5433"]
            Neo4j["Neo4j :7474/7687"]
            AI_KG["ai-kg :50051"]
            AI_CV["ai-cv :50052/8081"]
        end
    end

    User --> Cloudflare
    Cloudflare --> Backend
    Backend <--> Postgres
    Backend <--> Redis
    Backend <--> AI_KG
    Backend <--> AI_CV
    AI_KG <--> Neo4j
    AI_KG <--> AI_PG
    AI_CV <--> MinIO
```

---

## Docker Services

### docker-compose.prod.yml

| Service | Image | Ports | Health Check |
|---|---|---|---|
| postgres | postgres:15-alpine | 5432 | `pg_isready` |
| redis | redis:7-alpine | 6379 | `redis-cli ping` |
| minio | minio/minio | 9000, 9001 | None |
| ai-postgres | postgres:17 | 5433→5432 | `pg_isready` |
| neo4j | neo4j:5.18.1 | 7474, 7687 | `cypher-shell` |
| ai-kg | build: ../AI_server | 50051 | Socket check |
| ai-cv | build: ../AI_server/Computer_Vision | 50052, 8081 | Socket check |
| go-seeder | build: . | — | One-shot |
| go-backend | build: . | 8080 | `wget /ping` |
| cloudflared | cloudflare/cloudflared | 20241 | None |

---

## Environment Configuration

### Required Environment Variables

```bash
# Database
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your_secure_password
POSTGRES_DB=nutrix
DB_DSN=postgres://postgres:password@postgres:5432/nutrix?sslmode=disable

# Redis
REDIS_PASSWORD=nutrix-redis-secret

# Auth (for go-seeder)
JWT_SECRET=your-jwt-secret

# Security (CRITICAL - server won't start without these)
ENCRYPTION_KEYS={"v1":"your-32-byte-aes-encryption-key"}
ACTIVE_KEY_VERSION=v1
HMAC_KEY=your-32-byte-hmac-key

# AI Services
GRPC_AI_HOST=ai-kg
GRPC_AI_PORT=50051
GRPC_CV_HOST=ai-cv
GRPC_CV_PORT=50052

# External APIs
SPOONACULAR_API_KEY=your-key
RAPIDAPI_KEY=your-key

# Neo4j (for AI services)
NEO4J_USER=neo4j
NEO4J_PASSWORD=password123

# Cloudflare Tunnel
CLOUDFLARE_TUNNEL_TOKEN=your-tunnel-token
```

---

## Startup Sequence

```
1. postgres starts (healthy)
   ↓
2. redis starts (healthy)
   ↓
3. go-seeder runs (completes)
   ↓
4. ai-postgres starts (healthy)
   ↓
5. neo4j starts (healthy)
   ↓
6. ai-kg starts (healthy) ← waits for neo4j
   ↓
7. ai-cv starts (healthy) ← waits for neo4j + minio
   ↓
8. go-backend starts (healthy) ← waits for all above
   ↓
9. cloudflared starts ← waits for go-backend
```

---

## Database Setup

### PostgreSQL Configuration

- **Version**: PostgreSQL 15
- **Extensions**: `pg_trgm`, `unaccent`
- **Persistence**: Docker volume `nutrix_postgres_data`
- **Health Check**: `pg_isready`

### Redis Configuration

- **Version**: Redis 7
- **Persistence**: AOF enabled
- **Memory**: 256MB max with `allkeys-lru` eviction
- **Password**: Required
- **Health Check**: `redis-cli ping`

### Neo4j Configuration

- **Version**: Neo4j 5.18.1
- **Auth**: `neo4j/password123` (configurable)
- **Persistence**: Docker volume `nutrix_neo4j_data`
- **Hostname**: Fixed to `neo4j` for DNS resolution

---

## Secrets Management

### Critical Secrets

| Secret | Purpose | Storage | Rotation |
|---|---|---|---|
| ENCRYPTION_KEYS | AES-256 field encryption | Environment | Requires key version migration |
| HMAC_KEY | Blind index generation | Environment | Must update all indexes |
| JWT_SECRET | JWT signing | Environment | Graceful with token expiry |
| POSTGRES_PASSWORD | Database access | Environment | Must update connection string |
| REDIS_PASSWORD | Redis access | Environment | Must update connection string |
| NEO4J_PASSWORD | Neo4j access | Environment | Must update AI service config |

### Secret Rotation Process

1. Add new key version to `ENCRYPTION_KEYS`
2. Update `ACTIVE_KEY_VERSION` to new version
3. Migrate existing data to new key (background job)
4. Remove old key version after migration

---

## Logging

### Log Levels

| Level | Usage | Evidence |
|---|---|---|
| DEBUG | Detailed debugging | Development only |
| INFO | Normal operations | Production |
| WARN | Recoverable issues | Production |
| ERROR | Failures | Production |

### Structured Logging

```go
// Example log format
log.Printf("[SRE] KG target: %s", kgTarget)
log.Printf("[WATER] handler entered")
log.Printf("[DAILY_PLAN] requestedDate=%s", dateStr)
```

### Log Destinations

- **Stdout**: Container logs (docker logs)
- **Prometheus**: Metrics endpoint (/metrics)
- **CloudWatch/Stackdriver**: Requires external integration

---

## Monitoring

### Prometheus Metrics

**Endpoint**: `GET /metrics`

**Key Metrics**:

| Metric | Type | Description |
|---|---|---|
| `http_requests_total` | Counter | Total HTTP requests by method, path, status |
| `http_request_duration_seconds` | Histogram | Request latency |
| `redis_operation_duration_seconds` | Histogram | Redis operation latency |
| `grpc_request_duration_seconds` | Histogram | gRPC call latency |
| `planner_generation_failures_total` | Counter | Planner failures by reason |

**Dashboards**: Grafana (via `docker-compose.monitoring.yml`)

### Health Checks

| Check | Endpoint | Interval |
|---|---|---|
| API health | `GET /ping` | 10s |
| Prometheus | `GET /metrics` | 30s |
| PostgreSQL | `pg_isready` | 10s |
| Redis | `redis-cli ping` | 10s |
| Neo4j | `cypher-shell RETURN 1` | 15s |

---

## Scaling

### Horizontal Scaling

**Current State**: NOT SUPPORTED for planner
- Weekly plans cached in-memory only
- Multiple instances = inconsistent plan cache
- Requires Redis-backed plan storage for horizontal scaling

### Vertical Scaling

| Component | Default Resources | Scaling Path |
|---|---|---|
| go-backend | 1 CPU, 512MB RAM | Increase container limits |
| PostgreSQL | 1 CPU, 512MB RAM | Increase resources + connection pool |
| Redis | 256MB max | Increase maxmemory |
| Neo4j | 2 CPU, 2GB RAM | Increase JVM heap |

### GPU Scaling

- **ai-cv**: NVIDIA GPU support (optional)
- Requires `nvidia-docker` runtime
- Falls back to CPU without GPU

---

## Failure Recovery

### Database Recovery

1. **Point-in-time recovery**: Requires PostgreSQL backup + WAL archiving
2. **Current state**: No automated backup configured

### Redis Recovery

- **Persistence**: AOF (append-only file)
- **Recovery**: Redis auto-reloads AOF on restart
- **Data loss**: Last few seconds of data if crash

### AI Service Recovery

| Service | Recovery Action | RTO |
|---|---|---|
| neo4j crash | Docker auto-restart | ~30s |
| ai-kg crash | Docker auto-restart + health check | ~20s |
| ai-cv crash | Docker auto-restart + health check | ~20s |

---

## Backup & Recovery

### Current Backup State

| Data | Backup | RPO |
|---|---|---|
| PostgreSQL | ❌ Not configured | N/A |
| Redis | AOF only | Last write |
| MinIO | ❌ Not configured | N/A |
| Neo4j | ❌ Not configured | N/A |

### Recommended Backup Strategy

```yaml
# Add to docker-compose.prod.yml
services:
  postgres:
    volumes:
      - nutrix_postgres_backup:/backup
    command: >
      bash -c '
        pg_basebackup -h localhost -U postgres -D /backup -Ft -z -P &&
        find /backup -type f -mtime +7 -delete
      '
    environment:
      BACKUP_SCHEDULE: "0 2 * * *"
```

---

## Rollback Strategy

### Application Rollback

```bash
# 1. Stop current deployment
docker-compose -f docker-compose.prod.yml down

# 2. Use previous image tag
docker-compose -f docker-compose.prod.yml run --rm go-backend:previous

# 3. Update image tag
docker tag nutrix-backend:current nutrix-backend:previous
```

### Database Rollback

```bash
# 1. Stop application
docker-compose down

# 2. Restore from backup
docker-compose up -d postgres
docker exec nutrix-postgres-1 pg_restore -U postgres -d nutrix /backup/latest.tar.gz

# 3. Start application
docker-compose up -d
```

---

## Operational Runbooks

### Runbook: High CPU Usage

1. Check Prometheus: `node_cpu_usage > 80%`
2. Identify process: `docker stats`
3. Check logs: `docker logs go-backend`
4. Possible causes:
   - DoS attack → check rate limits
   - Slow queries → check PostgreSQL logs
   - AI service overload → check gRPC latency

### Runbook: Redis Connection Failure

1. Check Redis: `docker logs redis`
2. Verify network: `docker network inspect nutrix`
3. Check health: `docker exec redis redis-cli -a $REDIS_PASSWORD ping`
4. Impact: Rate limiting disabled, degraded mode

### Runbook: AI Service Unavailable

1. Check service: `docker logs ai-kg`
2. Check Neo4j: `docker logs neo4j`
3. Restart if needed: `docker-compose restart ai-kg`
4. Impact: KG features return UNKNOWN/ERROR

### Runbook: Database Slow Queries

1. Check slow query log: `docker exec postgres psql -U postgres -c "SELECT * FROM pg_stat_activity WHERE state = 'active'"`
2. Identify blocking: `docker exec postgres psql -U postgres -c "SELECT * FROM pg_locks"`
3. Kill slow query: `docker exec postgres psql -U postgres -c "SELECT pg_terminate_backend(pid)"`

---

## Deployment Checklist

- [ ] Environment variables configured
- [ ] Secrets generated and stored securely
- [ ] Database migrations run
- [ ] Food/DRIs seeded
- [ ] Health checks passing
- [ ] Prometheus metrics available
- [ ] Grafana dashboards accessible
- [ ] Cloudflare tunnel configured
- [ ] SSL certificate valid
- [ ] Backup strategy in place
- [ ] Monitoring alerts configured
- [ ] Runbook procedures documented

---

## Files

- Evidence: `docker-compose.prod.yml`
- Evidence: `docker-compose.monitoring.yml`
- Evidence: `Dockerfile`
- Evidence: `cmd/server/main.go`
