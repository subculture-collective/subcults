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
- Docker & Docker Compose (coming soon)

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

- `make build` - Build all Go binaries
- `make test` - Run all tests
- `make lint` - Run linters
- `make clean` - Remove build artifacts
- `make tidy` - Tidy Go modules
- `make fmt` - Format Go code

## Getting Started (Local Skeleton – Planned)

Documented in forthcoming issues: Docker Compose, Caddy reverse proxy, `.env.example` provisioning, migration scripts.

## License

To be defined. (Planned: permissive OSS; Apache-2.0 or MIT.)

## Contributing

Roadmap issues will guide implementation. Open discussion for refinements before large structural changes.
