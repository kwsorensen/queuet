.PHONY: help build test test-e2e run clean docker-build docker-run migrate migrate-down deps dev docker-dev lint

# Variables
APP_NAME=queuet
DOCKER_IMAGE=$(APP_NAME)
GO_FILES=$(shell find . -name "*.go" -not -path "./vendor/*")
GOLANGCI_LINT_VERSION=v1.55.2
GOPATH=$(shell go env GOPATH)
PATH:=$(PATH):$(GOPATH)/bin

# Help command to show available commands
help: ## Display this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk '/^[a-zA-Z_-]+:.*?## .*$$/ {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development commands
deps: ## Download Go dependencies
	go mod download
	go mod tidy

build: ## Build the application
	go build -o bin/$(APP_NAME) main.go

run: ## Run the application
	go run main.go

dev: ## Run the application in development mode with hot reload (requires air)
	which air || go install github.com/cosmtrek/air@latest
	air

clean: ## Clean build artifacts
	rm -rf bin/
	go clean

# Test commands
test: ## Run unit tests
	go test -v ./internal/...

test-e2e: ## Run end-to-end tests
	@echo "Starting test environment..."
	@echo "Cleaning up any existing test containers..."
	docker compose down -v || true
	@echo "Starting services with test ports..."
	TEST_DB_PORT=5433 TEST_REDIS_PORT=6380 docker compose up -d postgres redis || { echo "Failed to start services"; exit 1; }
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "Checking if services are running..."
	docker ps | grep -q queuet-postgres || { echo "PostgreSQL is not running"; exit 1; }
	docker ps | grep -q queuet-redis || { echo "Redis is not running"; exit 1; }
	@echo "Running migrations..."
	TEST_DB_PORT=5433 docker compose up --exit-code-from migrations migrations || { echo "Migrations failed"; docker compose down -v; exit 1; }
	@echo "Running E2E tests..."
	@POSTGRES_HOST=localhost \
	POSTGRES_PORT=5433 \
	POSTGRES_USER=postgres \
	POSTGRES_PASSWORD=postgres \
	POSTGRES_DB=queuet \
	REDIS_HOST=localhost \
	REDIS_PORT=6380 \
	go test -v ./tests/e2e/... || { echo "Tests failed"; docker compose down -v; exit 1; }
	@echo "Tests completed successfully"
	@echo "Cleaning up test environment..."
	docker compose down -v

test-coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint: ## Run linters
	@echo "Running go vet..."
	go vet ./...
	@echo "Running go fmt..."
	go fmt ./...
	@echo "Running go mod tidy..."
	go mod tidy

# Docker commands
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE) .

docker-run: ## Run Docker container
	docker run -p 8080:8080 $(DOCKER_IMAGE)

docker-dev: ## Start development environment with Docker Compose
	docker-compose up -d postgres redis
	docker-compose up migrations
	docker-compose up app

docker-down: ## Stop and remove Docker Compose services
	docker-compose down -v

# Database commands
migrate: ## Run database migrations
	docker-compose up migrations

migrate-create: ## Create a new migration file (requires NAME variable)
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Use 'make migrate-create NAME=your_migration_name'"; \
		exit 1; \
	fi
	@echo "Creating migration file for: $(NAME)"
	@touch migrations/$(shell date +%Y%m%d%H%M%S)_$(NAME).sql
	@echo "-- +migrate up\n\n-- +migrate down" > migrations/$(shell date +%Y%m%d%H%M%S)_$(NAME).sql

migrate-status: ## Show migration status
	docker-compose run --rm migrations tern status

migrate-down: ## Rollback the last migration
	docker-compose run --rm migrations tern migrate --destination -1

migrate-reset: ## Reset the database (rollback all migrations)
	docker-compose run --rm migrations tern migrate --destination 0

# Default target
default: help 