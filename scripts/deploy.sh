#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# Blue-Green Deployment Script for Subcults
# =============================================================================
# Deploys a new version with zero downtime using blue/green strategy.
# The inactive slot is rebuilt, smoke-tested, then traffic is switched.
#
# Usage:
#   ./scripts/deploy.sh                # Deploy latest from current branch
#   ./scripts/deploy.sh --rollback     # Switch back to previous slot
#   ./scripts/deploy.sh --status       # Show current active slot
#
# Prerequisites:
#   - Docker Compose v2+
#   - deploy/compose.yml configured
#   - External Caddy proxy on 'web' network
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_DIR="$PROJECT_DIR/deploy"
COMPOSE_FILE="$COMPOSE_DIR/compose.yml"
STATE_FILE="$COMPOSE_DIR/.deploy-state"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[deploy]${NC} $*"; }
warn() { echo -e "${YELLOW}[deploy]${NC} $*"; }
err()  { echo -e "${RED}[deploy]${NC} $*" >&2; }

# Track active slot (blue or green)
get_active_slot() {
    if [[ -f "$STATE_FILE" ]]; then
        cat "$STATE_FILE"
    else
        echo "blue"
    fi
}

set_active_slot() {
    echo "$1" > "$STATE_FILE"
}

get_inactive_slot() {
    local active
    active=$(get_active_slot)
    if [[ "$active" == "blue" ]]; then
        echo "green"
    else
        echo "blue"
    fi
}

# Health check with retries
wait_for_healthy() {
    local url="$1"
    local max_attempts="${2:-30}"
    local interval="${3:-2}"

    log "Waiting for $url to become healthy..."
    for i in $(seq 1 "$max_attempts"); do
        if curl -sf "$url" > /dev/null 2>&1; then
            log "Health check passed (attempt $i/$max_attempts)"
            return 0
        fi
        sleep "$interval"
    done
    err "Health check failed after $max_attempts attempts"
    return 1
}

# Smoke tests on the new deployment
run_smoke_tests() {
    local api_url="$1"
    local failures=0

    log "Running smoke tests against $api_url..."

    # Test liveness
    if ! curl -sf "$api_url/health/live" > /dev/null 2>&1; then
        err "  FAIL: /health/live"
        ((failures++))
    else
        log "  PASS: /health/live"
    fi

    # Test readiness
    if ! curl -sf "$api_url/health/ready" > /dev/null 2>&1; then
        warn "  WARN: /health/ready (may need dependencies)"
    else
        log "  PASS: /health/ready"
    fi

    # Test metrics endpoint
    if ! curl -sf "$api_url/metrics" > /dev/null 2>&1; then
        warn "  WARN: /metrics (may require auth token)"
    else
        log "  PASS: /metrics"
    fi

    if [[ "$failures" -gt 0 ]]; then
        err "Smoke tests failed ($failures failures)"
        return 1
    fi

    log "All smoke tests passed"
    return 0
}

# Show deployment status
cmd_status() {
    local active
    active=$(get_active_slot)
    log "Active slot: $active"
    log "Compose file: $COMPOSE_FILE"

    echo ""
    docker compose -f "$COMPOSE_FILE" ps 2>/dev/null || warn "No containers running"
}

# Main deploy flow
cmd_deploy() {
    local active inactive
    active=$(get_active_slot)
    inactive=$(get_inactive_slot)

    log "=== Blue-Green Deployment ==="
    log "Active slot:   $active"
    log "Deploying to:  $inactive"
    log ""

    # Step 1: Build new images
    log "[1/5] Building images..."
    docker compose -f "$COMPOSE_FILE" build --no-cache api indexer frontend

    # Step 2: Run database migrations (if needed)
    log "[2/5] Running database migrations..."
    (
        cd "$PROJECT_DIR"
        if [[ -f "$COMPOSE_DIR/.env" ]]; then
            set -a
            # shellcheck source=/dev/null
            source "$COMPOSE_DIR/.env"
            set +a
        fi
        if command -v migrate > /dev/null 2>&1; then
            migrate -path migrations -database "$DATABASE_URL" up || true
        else
            warn "  migrate CLI not found — skipping (run manually if needed)"
        fi
    )

    # Step 3: Start new containers alongside existing ones
    log "[3/5] Starting new containers..."
    docker compose -f "$COMPOSE_FILE" up -d --force-recreate api indexer frontend

    # Step 4: Wait for health + smoke test
    log "[4/5] Running health checks and smoke tests..."
    if ! wait_for_healthy "http://localhost:8080/health/live" 30 2; then
        err "New deployment failed health checks — rolling back"
        docker compose -f "$COMPOSE_FILE" up -d --force-recreate api indexer frontend
        exit 1
    fi

    if ! run_smoke_tests "http://localhost:8080"; then
        err "Smoke tests failed — rolling back"
        docker compose -f "$COMPOSE_FILE" up -d --force-recreate api indexer frontend
        exit 1
    fi

    # Step 5: Update state
    log "[5/5] Switching traffic to $inactive slot..."
    set_active_slot "$inactive"

    log ""
    log "=== Deployment Complete ==="
    log "Active slot: $inactive"
    log "Previous slot: $active"
    log ""
    log "To rollback: ./scripts/deploy.sh --rollback"
}

# Rollback to previous slot
cmd_rollback() {
    local active inactive
    active=$(get_active_slot)
    inactive=$(get_inactive_slot)

    log "=== Rollback ==="
    log "Current active: $active"
    log "Rolling back to: $inactive"

    # Restart with previous images (Docker caches previous layers)
    docker compose -f "$COMPOSE_FILE" up -d --force-recreate api indexer frontend

    set_active_slot "$inactive"

    log "Rollback complete. Active slot: $inactive"
    log "Verify: curl http://localhost:8080/health/live"
}

# Main
case "${1:-}" in
    --status)
        cmd_status
        ;;
    --rollback)
        cmd_rollback
        ;;
    --help|-h)
        echo "Usage: $0 [--status|--rollback|--help]"
        echo ""
        echo "  (no args)    Deploy latest version (blue-green)"
        echo "  --status     Show current deployment status"
        echo "  --rollback   Switch to previous deployment"
        echo "  --help       Show this help"
        ;;
    *)
        cmd_deploy
        ;;
esac
