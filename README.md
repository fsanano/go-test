# Go Backend Project

High-load HTTP Server.

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
   Start the database, run migrations, and automatically create `.env` file:
   ```bash
   make init
   ```

2. **Configure Credentials**
   Add your Skinport API key and Client ID to the `.env` file:
   ```bash
   SKINPORT_CLIENT_ID=your_client_id
   SKINPORT_API_KEY=your_api_key
   ```

3. **Run Application**
   Start the application locally with hot-reload:
   ```bash
   make run
   ```
   The application will start on port 8081.

### Commands
- `make init`: Start DB and apply migrations
- `make run`: Run the application locally (Air)
- `make build`: Build the binary
- `make up`: Start all services via Docker
- `make down`: Stop containers
- `make migration-create`: Create a new migration file
- `make migration-up`: Apply migrations
- `make migration-down`: Rollback migrations


### Features Implementation

This project implements the requirements as follows:

#### 1. Skinport Items Proxy (`GET /items`)
- **Integration**: Fetches items from `https://api.skinport.com/v1/items` with support for `app_id` and `currency`.
- **Concurrency**: Uses `errgroup` to fetch tradable and non-tradable items in parallel.
- **Caching**: Implements thread-safe in-memory caching with a 5-minute TTL to reduce API load.
- **Data Processing**: Merges tradable and non-tradable prices into a single object per item (MarketHashName), displaying minimum prices for both states.
- **Optimization**: Supports Brotli compression for efficient data transfer from Skinport.

#### 2. User Balance Deduction (`POST /buy`)
- **Architecture**: Clean Architecture (Handler -> Service -> Repository).
- **Transactional Consistency**: Uses PostgreSQL transactions (`RunAtomic`) to ensure atomic operations.
- **Concurrency Control**: Implements `SELECT ... FOR UPDATE` row-level locking for both user balance and item stock to prevent race conditions.
- **Validation**: Checks for sufficient funds and stock before processing.
- **Database**:
  - `users`: Stores user balance.
  - `items`: Stores item stock and price.
  - `orders`: Logs all successful purchases.

#### Additional Features
- **Configuration**: Environment variables via `.env` (auto-generated).
- **Database Access**: Uses `pgx/v5` with connection pooling (`pgxpool`) and no ORM.
- **Migrations**: Database schema managed by `goose`.
- **Hot Reload**: Configured `Air` for local development.
- **Docker**: Full `docker-compose` setup for PostgreSQL and the application.