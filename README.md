# NutriX Go Backend

**Module:** `nutrix-backend`
**Role:** REST API + gRPC client, deployment orchestrator

The NutriX backend provides a REST API for nutrition tracking and serves as the deployment orchestrator for the full NutriX stack across three repositories.

## 3-Repository Architecture

```
Bibiby/
|-- go_backend/     <- THIS REPO (REST API, deployment orchestrator)
|-- AI_server/      <- Python AI (ai-kg on Neo4j:50051, ai-cv on :50052)
|-- Nutrix/         <- Flutter client (web + Android)
```

- **Go backend** = source of truth for facts (users, foods, logs, PostgreSQL)
- **AI server** = source of truth for intelligence (Knowledge Graph, Computer Vision)
- **Cross-repo:** Production compose in go_backend builds AI_server services

## Quickstart

### Prerequisites

- Go 1.25+
- Docker and Docker Compose v2+

### Local Development

```bash
cd go_backend

# Build
GOWORK=off go build ./...

# Run tests
GOWORK=off go test -count=1 ./...

# Start local services (PostgreSQL, Redis, MinIO)
docker compose -f docker-compose.yml up -d
```

**Note:** All Go commands must use `GOWORK=off` to isolate from the parent `go.work` workspace.

## Docker Compose Matrix

| File | Role | Status | Services | Cross-repo deps |
|------|------|--------|----------|-----------------|
| `docker-compose.yml` | Dev/local | Active | postgres, redis, minio | None |
| `docker-compose.prod.yml` | Production | Active | Full stack (11 services) | Builds `../AI_server`, `../AI_server/Computer_Vision` |
| `docker-compose.edge.yml` | Static web edge | Active | nginx | Mounts `../NutriX/build/web` |
| `docker-compose.monitoring.yml` | Observability | Active/conditional | prometheus, grafana | None |
| `docker-compose.spark.yml` | Data pipeline | **Usage unverified** | spark-master, spark-worker | None |

## Environment Contract

### Development (`.env.example`)

### Production (`.env.server.example`)

Crash-critical variables that must be configured:
- `ENCRYPTION_KEYS` — User data encryption
- `GRPC_AI_HOST` — AI server hostname
- `GRPC_AI_PORT` — AI server gRPC port

See `.env.example` and `.env.server.example` for the full variable list.

**Note:** AI server-only variables (`GEMINI_API_KEY`, `OPENAI_API_KEY`) are not part of go_backend's environment contract.

## CI/CD

This repository uses GitHub Actions for continuous integration.

### Workflow

The CI pipeline runs on push to `main`/`deploy`/`dev` and on pull requests targeting `main`/`deploy`:

```yaml
name: CI

on:
  push:
    branches: [main, deploy, dev]
  pull_request:
    branches: [main, deploy]
```

### Pipeline Steps

1. **Build** — `go build ./...`
2. **Vet** — `go vet ./...`
3. **Test** — `go test -count=1 -race ./...`
4. **Lint** — `golangci-lint run` (see `.golangci.yml`)
5. **Compose validation** — All compose files validated via `docker compose config`

### Local Lint

```bash
golangci-lint run
```

Configuration is in `.golangci.yml` (conservative 7-linter config).

## Known Technical Debt

The following issues are tracked but not addressed in the current phase:

1. **`go.work` in parent directory** — CI and local dev use `GOWORK=off`. If a 3-repo Go workspace is desired, a separate phase is needed.

2. **`server.exe~` (51 MB) tracked in git** — `.gitignore` now covers `*.exe~`, but the existing tracked file remains.

3. **`lib/` contains Dart files** — `lib/main.dart`, `lib/utility/json_convert.dart` are wrong-language artifacts.

4. **`docker-compose.spark.yml` usage unverified** — Not referenced in any docs or deploy scripts.

5. **Local checkout on `deploy` branch** — Remote default is `main`.

## Development

### Module Structure

```
go_backend/
|-- cmd/           # Entry points
|-- internal/      # Private packages
|   |-- domain/    # Domain entities
|   |-- nutrition/  # Nutrition use cases
|   |-- user/      # User management
|   |-- product/   # Product catalog
|   |-- workout/   # Workout tracking
|   |-- infrastructure/
|       |-- grpc/  # gRPC client to AI server
|       |-- metrics/ # Prometheus metrics
|-- pkg/           # Public packages
|-- api/proto/     # gRPC protocol buffers
|-- config/        # Configuration
```

### Running Services

```bash
# Start all local services
docker compose up -d

# Start with monitoring
docker compose -f docker-compose.yml -f docker-compose.monitoring.yml up -d

# Production stack (cross-repo)
docker compose -f docker-compose.prod.yml up -d
```

## License

See parent repository.
