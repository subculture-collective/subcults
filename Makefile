.PHONY: help build build-api build-frontend test lint clean tidy verify fmt \
	migrate-up migrate-down compose-up compose-down

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
	@echo "Running frontend tests..."
	npm run test --if-present

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
	@./scripts/migrate.sh up

## migrate-down: Rollback the last database migration
migrate-down:
	@./scripts/migrate.sh down 1

## compose-up: Start all services with Docker Compose
compose-up:
	docker compose -f $(DOCKER_COMPOSE_FILE) up -d

## compose-down: Stop all services with Docker Compose
compose-down:
	docker compose -f $(DOCKER_COMPOSE_FILE) down
