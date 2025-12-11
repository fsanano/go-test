PROJECT_NAME := go-test

# Load .env file if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

build:
	@echo "Building..."
	@go build -o bin/http cmd/http/main.go

init: ## Init project (start db, migrate)
	@echo "Starting Database..."
	@docker-compose up -d db
	@echo "Waiting for Database to be ready..."
	@sleep 5
	@echo "Running Migrations..."
	@goose -dir migrations postgres "$(DATABASE_URL)" up || echo "Warning: Migrations failed or none found."
	@echo "Project initialized! Run 'make run' to start the app."

run: ## Run with hot-reload (requires air)
	@command -v air >/dev/null 2>&1 || (echo "Installing air..." && go install github.com/air-verse/air@v1.52.3)
	@command -v air >/dev/null 2>&1 || export PATH="$$PATH:$$(go env GOPATH)/bin"
	$$(go env GOPATH)/bin/air -c .air.toml

# Docker
docker-up:
	@echo "Starting services..."
	@docker-compose up -d --build

docker-down:
	@echo "Stopping services..."
	@docker-compose down

# Migrations
# Note: You need to install goose: go install github.com/pressly/goose/v3/cmd/goose@latest
migration-create:
	@read -p "Enter migration name: " name; \
	goose -dir migrations postgres "$(DATABASE_URL)" create $$name sql

migration-up:
	@echo "Running migrations..."
	@goose -dir migrations postgres "$(DATABASE_URL)" up

migration-down:
	@echo "Rolling back migrations..."
	@goose -dir migrations postgres "$(DATABASE_URL)" down

.PHONY: build run docker-up docker-down migration-create migration-up migration-down
