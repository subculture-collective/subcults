# Subcults Development Guide

## Project Overview

Subcults is a privacy-first underground music community platform combining Go backend services, React frontend, and AT Protocol integration for decentralized discovery. The core principle: **presence over popularity** with scene sovereignty and explicit location consent.

### Architecture

Three main services orchestrated via Docker Compose:
- **API** (`cmd/api`): Go/chi REST API handling scenes, events, payments (Stripe), auth (JWT), media (R2)
- **Indexer** (`cmd/indexer`): Jetstream consumer ingesting AT Protocol records into Postgres
- **Frontend** (`web/`): Vite + React + MapLibre for map-based discovery

**Critical Data Flow**: AT Protocol → Jetstream → Indexer → Postgres ← API ← Frontend

## Privacy & Security Patterns

### Location Consent (Core Invariant)

**ALL** scene/event location data must enforce consent via `allow_precise` flag:

```go
// Always call before persisting
scene.EnforceLocationConsent()  // Clears PrecisePoint if allow_precise=false
```

- Repository methods automatically enforce (see [internal/scene/repository.go](../internal/scene/repository.go))
- Database migrations default `allow_precise=FALSE` ([migrations/000001_add_allow_precise.up.sql](../migrations/000001_add_allow_precise.up.sql))
- Tests verify consent enforcement in both models and repositories

### Authentication

JWT-based auth with two token types ([internal/auth/jwt.go](../internal/auth/jwt.go)):
- **Access tokens**: 15min expiry, includes `did` claim (Decentralized Identifier)
- **Refresh tokens**: 7 day expiry, no DID
- Middleware injects `user_did` into context via `SetUserDID(ctx, did)`

### Request Tracing

All HTTP requests use structured logging with required fields ([internal/middleware/logging.go](../internal/middleware/logging.go)):
- `request_id` (via `X-Request-ID` header or auto-generated UUID)
- `method`, `path`, `status`, `latency_ms`, `size`
- `user_did` (if authenticated)
- `error_code` (for 4xx/5xx responses)

## Development Workflows

### Build & Test

```bash
make build          # All binaries to bin/
make build-api      # Just bin/api
make test           # Go tests (race detector + coverage)
make lint           # Go vet + frontend linters
```

### Database Migrations

Uses [golang-migrate](https://github.com/golang-migrate/migrate). Requires `DATABASE_URL` env var.

```bash
make migrate-up     # Apply all pending
make migrate-down   # Rollback last migration
./scripts/migrate.sh version  # Check current version
```

Auto-detects local `migrate` binary or falls back to Docker. See [scripts/migrate.sh](../scripts/migrate.sh) for advanced usage (force, drop, step counts).

### Docker Compose

```bash
make compose-up     # Start all services
make compose-down   # Stop all services
```

Override compose file: `make compose-up DOCKER_COMPOSE_FILE=docker-compose.dev.yml`

### Environment Setup

1. Copy `configs/dev.env.example` to `configs/dev.env`
2. Fill required secrets:
   - `DATABASE_URL`: Neon Postgres with PostGIS
   - `JWT_SECRET`: Min 32 chars (generate: `openssl rand -base64 32`)
   - `LIVEKIT_*`: WebRTC credentials for live audio
   - `STRIPE_*`: Payment integration
   - `R2_*`: Cloudflare R2 for media storage
   - `MAPTILER_API_KEY`: Map tiles

## Code Conventions

### Testing

- Table-driven tests with descriptive names: `TestScene_EnforceLocationConsent`
- Always test privacy invariants (see [internal/scene/scene_test.go](../internal/scene/scene_test.go))
- Use in-memory repositories for unit tests (thread-safe via `sync.RWMutex`)
- JWT tests use 44-char base64 secret matching `openssl rand -base64 32` output

### Go Style

- `internal/` for private app code (auth, middleware, domain models)
- `pkg/` for reusable packages (currently empty - use sparingly)
- `cmd/` for entry points (minimal logic, just flag parsing + initialization)
- Structured logging with `slog` (JSON in production, text in dev)
- Context values use private types (e.g., `type userDIDKey struct{}`)

### Error Handling

- Package-level sentinel errors: `var ErrInvalidToken = errors.New("...")`
- Context-aware error codes in responses (set via `SetErrorCode(ctx, "auth_failed")`)
- Middleware logs errors at appropriate levels (5xx=error, 4xx=warn)

### Dockerfiles

All services use multi-stage builds:
- **Build stage**: `golang:1.22-alpine` with cross-compilation support (`TARGETOS`/`TARGETARCH`)
- **Runtime stage**: `gcr.io/distroless/static-debian12:nonroot` for minimal attack surface
- CGO disabled for static binaries
- Target: <60MB images (current: ~3.4MB ✓)
- No secrets baked in - runtime env vars only

## Project Structure

```
cmd/             # Entry points (api, indexer, backfill)
internal/        # Private application code
  auth/          # JWT token management
  middleware/    # HTTP middleware (logging, request ID)
  scene/         # Domain models + repositories
migrations/      # Database schema changes
configs/         # Environment templates
scripts/         # Build/automation (migrate.sh)
web/             # Frontend (placeholder - Vite + React planned)
docs/            # Documentation (docker.md)
perf/            # Performance baselines
```

## Roadmap Context

Currently in **Phase 0 (Foundations)**:
- Containerized stack ✓
- Core schema (scenes/events with location consent) ✓
- Auth scaffolding ✓
- Migration tooling ✓

**Active Development (200+ GitHub issues)**:
The project uses detailed GitHub issues for task tracking. Major epics include:
- **Backend Core** (#3): Chi router setup, handlers, config management
- **Jetstream Indexer** (#5): AT Protocol real-time ingestion via WebSocket
- **Scene/Event/Post Management** (#10, #15, #17): CRUD APIs for core entities
- **Trust Graph** (#24): Alliance system + ranking algorithm (feature flagged)
- **LiveKit Streaming** (#23): WebRTC audio rooms with participant controls
- **Mapping Frontend** (#14): MapLibre + clustering with privacy jitter visualization
- **Stripe Connect** (#22): Direct scene payouts with platform fees
- **Security & Hardening** (#20): Rate limiting, CORS, secret rotation
- **Observability** (#19): Prometheus metrics, OpenTelemetry tracing, dashboards
- **Performance** (#17): Query optimization, caching strategies, latency budgets
- **Documentation** (#12): Architecture docs, API reference, onboarding guides

See issues for detailed acceptance criteria and step-by-step implementation plans.

## Common Pitfalls

1. **Never persist location data without calling `EnforceLocationConsent()`** - repositories enforce this but models allow mutation
2. **Middleware ordering matters**: `RequestID` → `Logging` (logging needs request ID in context)
3. **Migration scripts need DATABASE_URL** - export before running `make migrate-*`
4. **Frontend is currently placeholder** - `npm run build` creates stub dist/index.html (see Epic #21 for app shell work)
5. **In-memory repos return deep copies** - prevents external mutation, but wastes memory in production (use Postgres repo)
6. **Chi router not yet initialized** - `cmd/api/main.go` is skeleton (see Epic #3 for handler implementation)
7. **Jetstream indexer is TODO** - `cmd/indexer/main.go` awaits WebSocket client implementation (Epic #5)
8. **Issue templates guide implementation** - check GitHub issues for detailed steps and acceptance criteria before starting work

## External Dependencies

- **Neon Postgres 16**: Primary database with PostGIS for geo queries
- **AT Protocol/Jetstream**: Decentralized identity and data ingestion
- **LiveKit Cloud**: WebRTC SFU for live audio rooms
- **Stripe Connect**: Direct scene payouts with platform fee
- **Cloudflare R2**: S3-compatible media storage
- **MapTiler**: Map tiles for MapLibre frontend
