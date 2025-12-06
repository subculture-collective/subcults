# Subcult

Subcult connects underground and local music communities by mapping scenes, events, and live audio sessions while preserving autonomy, privacy, and creative identity.

## Vision

Rebuild the connective tissue of the underground: a trust‑based discovery layer (not a follower feed) where artists, venues, collectives, and curators surface what is happening around them without algorithmic flattening.

## Core Pillars

1. Presence over popularity
2. Scene sovereignty (custom identity & membership rules)
3. Human discovery (proximity + trust > opaque ranking)
4. Decentralized data (AT Protocol records + Jetstream ingestion)
5. Privacy first (coarse location, consent‑based precision)

## Initial Stack

- Frontend: Vite + React + TypeScript + MapLibre (MapTiler tiles)
- Backend: Go (chi) API + Jetstream indexer
- RTC Audio: LiveKit Cloud (WebRTC SFU, TURN, token issuance)
- Database: Neon Postgres 16 + PostGIS (geo + FTS)
- Storage: Cloudflare R2 (media assets, recordings)
- Payments: Stripe Connect (direct scene payouts, platform fee)

## Early Features (MVP)

- Create & manage scenes (visual identity, membership)
- Publish events & posts (flyers, mixes, releases)
- Map-based discovery (nearby scenes/events, clustering)
- Live audio sessions (room join, host/guest roles)
- Basic trust graph (memberships + alliances scoring)
- Coarse location privacy & EXIF stripping
- Direct revenue (ticket/merch checkout)

## Roadmap Phases

| Phase | Focus | Key Outcomes |
|-------|-------|--------------|
| 0 | Foundations | Containerized stack, core schema, auth, config |
| 1 | MVP Core | Scenes, events, map discovery, streaming, payments |
| 2 | Growth & Trust | Alliances, ranking, moderation, observability |
| 3 | Scale & Performance | OpenSearch option, mobile app alignment, backfills |

## Development Principles

- Small, self‑contained issues (actionable, testable, reversible)
- Explicit acceptance criteria & privacy considerations per feature
- Observability baked in (structured logs + metrics + traces)
- Security & safety reviews precede public feature exposure

## Project Structure

```
subcults/
├── cmd/
│   ├── api/          # Main API server entry point
│   └── backfill/     # Backfill command for data migration
├── deploy/           # Docker Compose and deployment configs
├── internal/         # Private application code
├── pkg/              # Reusable packages
├── web/              # Frontend application (Vite + React)
├── scripts/          # Build and automation scripts
├── docs/             # Documentation files
├── migrations/       # Database migration files
├── configs/          # Configuration templates
└── perf/             # Performance baselines and reports
```

## Getting Started

### Prerequisites

- Go 1.21+
- Node.js 18+
- Docker & Docker Compose

### Setup

1. Copy environment configuration:

   ```bash
   cp configs/dev.env.example configs/dev.env
   # Edit configs/dev.env with your values
   ```

2. Install dependencies:

   ```bash
   go mod tidy
   npm install
   ```

3. Build the project:

   ```bash
   make build
   ```

4. Run tests:

   ```bash
   make test
   ```

### Available Make Targets

Run `make help` to see all available targets:

#### Build Targets
- `make build` - Build all Go binaries
- `make build-api` - Build only the API binary (outputs to `bin/api`)
- `make build-frontend` - Build the frontend application (outputs to `dist/`)

#### Test & Lint
- `make test` - Run all tests (Go and frontend if available)
- `make lint` - Run linters (Go vet and frontend linters)

#### Code Quality
- `make fmt` - Format Go code
- `make tidy` - Tidy Go modules
- `make verify` - Verify Go modules
- `make clean` - Remove build artifacts

#### Database
- `make migrate-up` - Apply all pending database migrations
- `make migrate-down` - Rollback the last database migration

#### Docker Compose
- `make compose-up` - Start all services with Docker Compose
- `make compose-down` - Stop all services with Docker Compose

You can customize the Docker Compose file path using the `DOCKER_COMPOSE_FILE` variable:

```bash
make compose-up DOCKER_COMPOSE_FILE=docker-compose.dev.yml
```

### Full Stack with Docker Compose

The `deploy/compose.yml` provides a complete local development stack with Caddy reverse proxy, API, indexer, and frontend:

```bash
# Navigate to the deploy directory
cd deploy

# Copy and configure environment variables
cp ../configs/dev.env.example .env
# Edit .env with your values

# Start all services
docker compose up -d

# Verify services are healthy
docker compose ps

# View logs
docker compose logs -f

# Stop all services
docker compose down
```

**Services:**
- **Caddy** (ports 80/443): Reverse proxy serving static assets and proxying to API
- **API** (internal): Go backend with health check at `/health`
- **Indexer** (internal): Jetstream consumer for AT Protocol ingestion
- **Web Build**: One-time service that builds and copies frontend assets

**Networks:**
- `proxy`: External-facing network for Caddy
- `internal`: Internal network for API and Indexer (not exposed to host)

**Volumes:**
- `web-dist`: Shared volume for frontend static assets
- `caddy_data`: Caddy TLS certificates and state
- `caddy_config`: Caddy configuration

### Database Migrations

Database schema changes are managed using [golang-migrate](https://github.com/golang-migrate/migrate). Migrations are stored in the `migrations/` directory.

#### Running Migrations

The migration commands require `DATABASE_URL` environment variable to be set:

```bash
export DATABASE_URL='postgres://user:pass@localhost:5432/subcults?sslmode=disable'
```

**Using Make targets (recommended):**

```bash
# Apply all pending migrations
make migrate-up

# Rollback the last migration
make migrate-down
```

**Using the migration script directly:**

Apply all pending migrations:

```bash
# Make the script executable (first time only)
chmod +x scripts/migrate.sh

# Run migrations
./scripts/migrate.sh up
```

Alternatively, you can run the script with `bash`:

```bash
bash scripts/migrate.sh up
```

Apply a specific number of migrations:

```bash
./scripts/migrate.sh up 1
```

Rollback the last migration:

```bash
./scripts/migrate.sh down 1
```

Check current migration version:

```bash
./scripts/migrate.sh version
```

The script automatically uses either the local `migrate` binary (if installed) or falls back to Docker.

## Privacy

Subcult is built with privacy as a core principle. See [docs/PRIVACY.md](docs/PRIVACY.md) for technical details on:

- Location consent controls and coarse geohash handling
- Media sanitization (EXIF stripping)
- Access logging practices
- User authentication and rate limiting

## License

To be defined. (Planned: permissive OSS; Apache-2.0 or MIT.)

## Contributing

Roadmap issues will guide implementation. Open discussion for refinements before large structural changes.
