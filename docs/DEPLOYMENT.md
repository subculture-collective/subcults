# Deployment Guide

## Overview

Subcults runs on a VPS with Docker Compose, Caddy as reverse proxy, and a centralized monitoring stack. This document covers the deployment architecture and procedures.

## Architecture

```
Internet → Caddy (TLS + routing) → Docker Compose services
                                     ├── subcults-api:8080
                                     ├── subcults-indexer
                                     └── subcults-frontend:80

Monitoring stack (~/projects/monitoring):
  Prometheus → scrapes subcults-api:8080/metrics
  Prometheus → scrapes subcults-indexer:9090/internal/indexer/metrics
  Grafana → dashboards (api-overview, indexer, streaming-trust, frontend-telemetry)
  Loki + Promtail → log aggregation
  Jaeger → distributed tracing (OTLP)
  Alertmanager → alerts to Slack/PagerDuty
```

## Prerequisites

1. **Docker & Docker Compose v2+** on the VPS
2. **Caddy** running at `~/projects/caddy` on the `web` Docker network
3. **Monitoring** running at `~/projects/monitoring` on the `web` and `monitoring` networks
4. **Neon Postgres** (managed, external) with PostGIS
5. **Environment file**: `deploy/.env` with all secrets populated

## Environment Setup

```bash
# Create the shared Docker network (if not exists)
docker network create web 2>/dev/null || true

# Copy and fill environment file
cp configs/dev.env.example deploy/.env
# Edit deploy/.env with production values
```

Required environment variables — see `configs/dev.env.example` for the full list.

## Standard Deployment

### Using the deploy script (recommended)

```bash
# Deploy latest version
./scripts/deploy.sh

# Check status
./scripts/deploy.sh --status

# Rollback if needed
./scripts/deploy.sh --rollback
```

### Manual deployment

```bash
cd deploy/

# Pull latest code
git pull origin main

# Build images
docker compose build --no-cache

# Run migrations (if any)
cd .. && make migrate-up && cd deploy/

# Recreate services
docker compose up -d --force-recreate

# Verify
curl -s http://localhost:8080/health/live
curl -s http://localhost:8080/health/ready
```

## Pre-deployment Checklist

Before deploying to production:

- [ ] All CI checks passing (unit tests, lint, build)
- [ ] Integration tests passing
- [ ] Coverage gates met (80%+ backend, 70%+ frontend)
- [ ] Security scan clean (no CRITICAL vulnerabilities)
- [ ] Database migrations tested on staging/dev
- [ ] Feature flags verified for new features
- [ ] Rollback plan documented
- [ ] Monitoring dashboards accessible
- [ ] Alert rules configured for new endpoints/services

## Docker Images

| Service  | Dockerfile            | Base Image                              | Port |
| -------- | --------------------- | --------------------------------------- | ---- |
| API      | `Dockerfile.api`      | golang:1.24-alpine → alpine:3.21        | 8080 |
| Indexer  | `Dockerfile.indexer`  | golang:1.24-alpine → distroless:nonroot | -    |
| Frontend | `Dockerfile.frontend` | node:22-alpine → nginx:1-alpine-slim    | 80   |

Build manually:

```bash
make docker-build              # Build all
make docker-build-api          # API only
make docker-build-frontend     # Frontend only
make docker-build-indexer      # Indexer only
```

## Networking

All Subcults containers join two Docker networks:

- **`web`** (external): Shared with Caddy reverse proxy. Caddy routes `subcults.subcult.tv` traffic to the appropriate service.
- **`subcults-internal`** (bridge): Inter-service communication (API ↔ Indexer).

Caddy configuration is at `~/projects/caddy/conf.d/subcults.subcult.tv.caddy`.

## Monitoring Integration

The monitoring stack at `~/projects/monitoring` is already configured:

- **Prometheus** scrapes `subcults-api:8080/metrics` and `subcults-indexer:9090/internal/indexer/metrics` every 15s
- **Grafana** has 4 provisioned dashboards:
  - `api-overview` — latency, throughput, error rates
  - `indexer` — processing lag, message rates, reconnections
  - `streaming-trust` — stream quality, trust recompute, audio metrics
  - `frontend-telemetry` — client errors, performance metrics
- **Alertmanager** routes alerts with subcults-specific rules in `prometheus/alerts/subcults.yml`
- **Loki** collects logs from all Docker containers via Promtail
- **Jaeger** receives traces via OTLP (port 4317 gRPC, 4318 HTTP)

Access monitoring at `https://sentinel.subcult.tv`.

## Rollback Procedure

If a deployment causes issues:

1. **Immediate rollback** (< 5 minutes):

   ```bash
   ./scripts/deploy.sh --rollback
   ```

2. **Manual rollback** to a specific commit:

   ```bash
   git checkout <known-good-commit>
   cd deploy && docker compose build && docker compose up -d --force-recreate
   ```

3. **Database rollback** (if migration caused issues):
   ```bash
   make migrate-down  # Rolls back last migration
   ```

## Kubernetes (Future)

K8s manifests are in `deploy/k8s/` and a Helm chart is in `deploy/helm/subcults/`. These are prepared for when the project scales beyond a single VPS.

```bash
# Lint Helm chart
helm lint deploy/helm/subcults/

# Render templates
helm template subcults deploy/helm/subcults/ -f deploy/helm/subcults/values-prod.yaml

# Deploy to a cluster
helm install subcults deploy/helm/subcults/ -f deploy/helm/subcults/values-prod.yaml -n subcults
```

## Troubleshooting

### Service won't start

```bash
docker compose -f deploy/compose.yml logs api
docker compose -f deploy/compose.yml logs indexer
```

### Health check failing

```bash
# Check liveness
curl -v http://localhost:8080/health/live

# Check readiness (includes dependency checks)
curl -v http://localhost:8080/health/ready
```

### Database connection issues

```bash
# Test connectivity
psql "$DATABASE_URL" -c "SELECT 1"

# Check migration state
./scripts/migrate.sh version
```

### Caddy not routing traffic

```bash
# Check Caddy logs
docker logs caddy --tail 50

# Verify DNS resolves
dig subcults.subcult.tv

# Check containers are on web network
docker network inspect web | grep subcults
```
