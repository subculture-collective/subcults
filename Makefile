.PHONY: help build build-api build-frontend test test-e2e test-load lint clean tidy verify fmt \
	migrate-up migrate-down compose-up compose-down logs dev dev-api dev-frontend dev-indexer

# Default target
.DEFAULT_GOAL := help

# Docker Compose configuration
DOCKER_COMPOSE_FILE ?= docker-compose.yml

## help: Display this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build all Go binaries
build:
	@echo "Building Go binaries..."
	go build -o bin/api ./cmd/api
	go build -o bin/indexer ./cmd/indexer
	go build -o bin/backfill ./cmd/backfill

## build-api: Build only the API binary
build-api:
	@echo "Building API binary..."
	go build -o bin/api ./cmd/api

## build-frontend: Build the frontend application
build-frontend:
	@echo "Building frontend..."
	npm run build

## test: Run all tests (Go and frontend if available)
test:
	@echo "Running Go tests..."
	go test -v -race -cover ./...
	@echo "Running frontend tests (if defined in package.json)..."
	npm run test --if-present

## test-e2e: Run E2E tests for streaming functionality
test-e2e:
	@echo "Running E2E tests..."
	npm run test:e2e

## test-load: Run k6 load tests for streaming
test-load:
	@echo "Running load tests..."
	npm run test:load

## lint: Run linters
lint:
	@echo "Running Go linters..."
	go vet ./...
	@echo "Running frontend linters..."
	npm run lint --if-present

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf dist/
	rm -rf coverage.out

## tidy: Tidy Go modules
tidy:
	go mod tidy

## verify: Verify Go modules
verify:
	go mod verify

## fmt: Format Go code
fmt:
	go fmt ./...

## migrate-up: Apply all pending database migrations
migrate-up:
	@set -a && . ./configs/dev.env && set +a && ./scripts/migrate.sh up

## migrate-down: Rollback the last database migration
migrate-down:
	@set -a && . ./configs/dev.env && set +a && ./scripts/migrate.sh down 1

## compose-up: Start all services with Docker Compose
compose-up:
	@test -f $(DOCKER_COMPOSE_FILE) || (echo "Error: $(DOCKER_COMPOSE_FILE) not found" && exit 1)
	docker compose -f $(DOCKER_COMPOSE_FILE) up -d

## compose-down: Stop all services with Docker Compose
compose-down:
	@test -f $(DOCKER_COMPOSE_FILE) || (echo "Error: $(DOCKER_COMPOSE_FILE) not found" && exit 1)
	docker compose -f $(DOCKER_COMPOSE_FILE) down

## dev: Run API and frontend development servers (requires: compose-up, database migrations)
dev:
	@echo "Starting development servers..."
	@echo "Ensure you've run 'make compose-up' and 'make migrate-up' first"
	@trap 'kill %1 %2' EXIT; \
	make dev-api & \
	make dev-frontend & \
	wait

## dev-api: Run API server with hot reload (requires: compose-up, migrations)
dev-api:
	@set -a && . ./configs/dev.env && set +a && \
	echo "Starting API server on http://$$HOST:$$PORT (from configs/dev.env)" && \
	go run ./cmd/api

## dev-frontend: Run frontend development server
dev-frontend:
	@echo "Starting frontend dev server..."
	@cd web && npm run dev

## dev-indexer: Run Jetstream indexer (requires: compose-up, migrations)
dev-indexer:
	@set -a && . ./configs/dev.env && set +a && \
	echo "Starting Jetstream indexer..." && \
	go run ./cmd/indexer

## logs: Stream Docker Compose logs from all services
logs:
	@test -f $(DOCKER_COMPOSE_FILE) || (echo "Error: $(DOCKER_COMPOSE_FILE) not found" && exit 1)
	docker compose -f $(DOCKER_COMPOSE_FILE) logs -f

## logs-api: Stream logs from API service only
logs-api:
	@test -f $(DOCKER_COMPOSE_FILE) || (echo "Error: $(DOCKER_COMPOSE_FILE) not found" && exit 1)
	docker compose -f $(DOCKER_COMPOSE_FILE) logs -f api

## logs-postgres: Stream logs from PostgreSQL service only
logs-postgres:
	@test -f $(DOCKER_COMPOSE_FILE) || (echo "Error: $(DOCKER_COMPOSE_FILE) not found" && exit 1)
	docker compose -f $(DOCKER_COMPOSE_FILE) logs -f postgres
