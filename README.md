# Go Backend Project

High-load service backend scaffold for a trading company.

## Tech Stack
- Go 1.23+
- PostreSQL 15
- go-chi (Router)
- pgx (Database Driver)
- goose (Migrations)

## Prerequisites
- Go 1.23+
- Docker & Docker Compose
- Make

## Getting Started

### Local Development

1. **Initialize Project**
   Start the database and run migrations:
   ```bash
   make init
   ```

2. **Run Application**
   Start the application locally with hot-reload:
   ```bash
   make run
   ```
   The application will start on port 8081.

### Commands
- `make init`: Start DB and apply migrations
- `make run`: Run the application locally (Air)
- `make build`: Build the binary
- `make docker-up`: Start all services via Docker
- `make docker-down`: Stop containers
- `make migration-create`: Create a new migration file
- `make migration-up`: Apply migrations
- `make migration-down`: Rollback migrations
