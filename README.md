# Queuet

[![Tests](https://github.com/queuet/queuet/actions/workflows/test.yml/badge.svg)](https://github.com/queuet/queuet/actions/workflows/test.yml)
[![Security](https://github.com/queuet/queuet/actions/workflows/security.yml/badge.svg)](https://github.com/queuet/queuet/actions/workflows/security.yml)

A RESTful API service built with Go, using PostgreSQL for data storage and Redis for caching.

## Features

- RESTful API endpoints for task management
- PostgreSQL database integration
- Redis caching
- Docker support
- Unit and E2E tests
- Database migrations using Tern
- Makefile for common operations
- Comprehensive test coverage
- Docker support with multi-arch images
- Automated security scanning
- CI/CD with GitHub Actions

## Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Make
- PostgreSQL 15
- Redis 7

## Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/queuet.git
   cd queuet
   ```

2. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```

3. Start the development environment:
   ```bash
   make docker-dev
   ```

   This will:
   - Start PostgreSQL and Redis
   - Run database migrations
   - Start the application

The API will be available at `http://localhost:8080`.

## Using Make Commands

The project includes a Makefile with common commands. View all available commands:
```bash
make help
```

### Common Commands

#### Development
- `make deps` - Download Go dependencies
- `make build` - Build the application
- `make run` - Run the application
- `make dev` - Run with hot reload (requires air)
- `make lint` - Run linters

#### Testing
- `make test` - Run unit tests
- `make test-e2e` - Run end-to-end tests
- `make test-coverage` - Run tests with coverage report

#### Docker Operations
- `make docker-build` - Build Docker image
- `make docker-run` - Run Docker container
- `make docker-dev` - Start development environment
- `make docker-down` - Stop and remove services

#### Database Migrations
- `make migrate` - Run migrations
- `make migrate-create NAME=your_migration` - Create a new migration
- `make migrate-status` - Show migration status
- `make migrate-down` - Rollback last migration
- `make migrate-reset` - Reset database (rollback all)

## Database Migrations

The project uses Tern for managing database migrations. All migrations are stored in the `migrations` directory.

### Creating a New Migration

```bash
make migrate-create NAME=add_user_table
```

This will create a new migration file with the current timestamp in the `migrations` directory.

Edit the migration file with your SQL:
```sql
-- +migrate up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE
);

-- +migrate down
DROP TABLE IF EXISTS users;
```

Run the migration:
```bash
make migrate
```

## API Endpoints

- `GET /health` - Health check endpoint
- `GET /api/v1/tasks` - List all tasks
- `POST /api/v1/tasks` - Create a new task
- `GET /api/v1/tasks/{id}` - Get a specific task
- `PUT /api/v1/tasks/{id}` - Update a task
- `DELETE /api/v1/tasks/{id}` - Delete a task

## Development

### Local Development

1. Install dependencies:
   ```bash
   make deps
   ```

2. Start the development environment:
   ```bash
   make docker-dev
   ```

   Or for local development with hot reload:
   ```bash
   make dev
   ```

## Project Structure

```
.
├── Dockerfile
├── Dockerfile.migrations
├── Makefile
├── README.md
├── docker-compose.yml
├── go.mod
├── go.sum
├── main.go
├── migrations/
│   ├── tern.conf
│   └── 001_create_tasks_table.sql
├── scripts/
│   └── run-migrations.sh
├── internal/
│   ├── handlers/
│   ├── models/
│   ├── database/
│   ├── cache/
│   └── routes/
└── tests/
    └── e2e/
```

## License

MIT 

## CI/CD

The project uses GitHub Actions for continuous integration and delivery:

### Workflows

1. **Tests (`test.yml`)**
   - Runs on PRs and pushes to main
   - Linting with golangci-lint
   - Unit and integration tests
   - Coverage reporting

2. **Security (`security.yml`)**
   - Runs on PRs, pushes to main, and weekly
   - Trivy vulnerability scanning
   - Gosec security scanning
   - Dependency review
   - SARIF report generation

3. **Release (`release.yml`)**
   - Runs on semver tags (e.g., v1.0.0)
   - Multi-arch Docker image building
   - Container security scanning
   - Publishing to GitHub Container Registry

### Docker Images

Docker images are automatically built and published to GitHub Container Registry on semver tags.

To use the latest version:
```bash
docker pull ghcr.io/queuet/queuet:latest
```

Or a specific version:
```bash
docker pull ghcr.io/queuet/queuet:v1.0.0
```

### Release Process

1. Create and push a new tag:
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. The release workflow will automatically:
   - Build multi-arch Docker images
   - Run security scans
   - Push to GitHub Container Registry with version tags 