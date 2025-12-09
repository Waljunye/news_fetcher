.PHONY: build run clean test vet fmt lint migrate-up migrate-down migrate-create docker-up docker-down docker-logs help

# Application
APP_NAME := news_fetcher
BUILD_DIR := bin
MAIN_PATH := cmd/syncer/main.go

# Database
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_NAME ?= news_fetcher
DB_SSL_MODE ?= disable
DB_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)

build:
	@echo "Building $(APP_NAME)..."
	@go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_PATH)

run: build
	@echo "Running $(APP_NAME)..."
	@./$(BUILD_DIR)/$(APP_NAME) -config config.yaml

run-dev:
	@go run $(MAIN_PATH) -config config.yaml

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)

test:
	@echo "Running tests..."
	@go test -v ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html

vet:
	@echo "Running go vet..."
	@go vet ./...

fmt:
	@echo "Formatting code..."
	@go fmt ./...

lint:
	@echo "Running linter..."
	@golangci-lint run ./...

deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

migrate-up:
	@echo "Running migrations up..."
	@migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	@echo "Running migrations down..."
	@migrate -path migrations -database "$(DB_URL)" down

migrate-down-one:
	@echo "Rolling back one migration..."
	@migrate -path migrations -database "$(DB_URL)" down 1

migrate-create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name

migrate-version:
	@migrate -path migrations -database "$(DB_URL)" version

migrate-force:
	@read -p "Enter version to force: " version; \
	migrate -path migrations -database "$(DB_URL)" force $$version

install-tools:
	@echo "Installing tools..."
	@go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

docker-up:
	@echo "Starting PostgreSQL..."
	@docker compose up -d

docker-down:
	@echo "Stopping PostgreSQL..."
	@docker compose down

docker-logs:
	@docker compose logs -f

docker-reset:
	@echo "Resetting PostgreSQL..."
	@docker compose down -v
	@docker compose up -d

start: docker-up
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@$(MAKE) migrate-up
	@$(MAKE) run-dev

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build & Run:"
	@echo "  build          Build the application"
	@echo "  run            Build and run"
	@echo "  run-dev        Run with go run"
	@echo "  start          Start DB + migrate + run"
	@echo "  clean          Remove build artifacts"
	@echo ""
	@echo "Testing:"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage"
	@echo "  vet            Run go vet"
	@echo "  fmt            Format code"
	@echo "  lint           Run golangci-lint"
	@echo ""
	@echo "Database:"
	@echo "  migrate-up     Apply migrations"
	@echo "  migrate-down   Rollback all migrations"
	@echo "  migrate-create Create new migration"
	@echo ""
	@echo "Docker:"
	@echo "  docker-up      Start PostgreSQL container"
	@echo "  docker-down    Stop PostgreSQL container"
	@echo "  docker-logs    View container logs"
	@echo "  docker-reset   Reset DB (delete volume)"
	@echo ""
	@echo "Tools:"
	@echo "  install-tools  Install migrate and golangci-lint"
	@echo "  deps           Download dependencies"
