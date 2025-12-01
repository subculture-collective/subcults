.PHONY: help build test lint clean

# Default target
.DEFAULT_GOAL := help

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
	go build -o bin/backfill ./cmd/backfill

## test: Run all tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

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
