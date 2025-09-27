# Pushkinlib Makefile

.PHONY: help build run test clean docker-build docker-run docker-stop docker-clean generate-catalog

# Default target
help:
	@echo "Pushkinlib - Book Library Service"
	@echo ""
	@echo "Available targets:"
	@echo "  build              Build binaries"
	@echo "  run                Run the service locally"
	@echo "  test               Run tests"
	@echo "  clean              Clean build artifacts"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build       Build Docker image"
	@echo "  docker-run         Run with docker-compose"
	@echo "  docker-stop        Stop docker-compose services"
	@echo "  docker-clean       Clean Docker images and volumes"
	@echo "  docker-logs        Show Docker logs"
	@echo ""
	@echo "Catalog targets:"
	@echo "  generate-catalog   Generate INPX catalog from books"
	@echo "  generate-test      Generate test catalog"
	@echo ""
	@echo "Production targets:"
	@echo "  docker-prod        Run production stack"
	@echo "  docker-prod-stop   Stop production stack"

# Build targets
build:
	@echo "Building Pushkinlib..."
	CGO_ENABLED=1 go build -tags sqlite_fts5 -o pushkinlib ./cmd/pushkinlib
	CGO_ENABLED=1 go build -tags sqlite_fts5 -o catalog-generator ./cmd/catalog-generator

run: build
	@echo "Starting Pushkinlib..."
	./pushkinlib

test:
	@echo "Running tests..."
	CGO_ENABLED=1 go test -tags sqlite_fts5 ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -f pushkinlib catalog-generator
	rm -rf cache/

# Docker targets
docker-build:
	@echo "Building Docker image..."
	docker build -t pushkinlib:latest .

docker-run: docker-stop
	@echo "Starting Pushkinlib with Docker Compose..."
	docker-compose up -d
	@echo "Services started. Access:"
	@echo "  Web interface: http://localhost:9090"
	@echo "  OPDS catalog:  http://localhost:9090/opds"
	@echo "  Health check:  http://localhost:9090/health"

docker-stop:
	@echo "Stopping Docker Compose services..."
	docker-compose down

docker-clean: docker-stop
	@echo "Cleaning Docker images and volumes..."
	docker-compose down -v --rmi all
	docker system prune -f

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose logs -f pushkinlib

# Production targets
docker-prod:
	@echo "Starting production stack..."
	docker-compose -f docker-compose.prod.yml up -d
	@echo "Production services started"

docker-prod-stop:
	@echo "Stopping production stack..."
	docker-compose -f docker-compose.prod.yml down

# Catalog generation targets
generate-catalog:
	@echo "Generating INPX catalog..."
	./catalog-generator -books=./sample-data/books -output=./sample-data -name=library

generate-test:
	@echo "Generating test catalog..."
	./catalog-generator -books=./sample-data/books -output=./sample-data -name=testlib -max-books=100

# Development targets
dev-setup:
	@echo "Setting up development environment..."
	go mod download
	go mod tidy

dev-run:
	@echo "Running in development mode..."
	INPX_PATH=./sample-data/testlib.inpx BOOKS_DIR=./sample-data ./pushkinlib

# Database management
db-clean:
	@echo "Cleaning database..."
	rm -f cache/pushkinlib.db*

db-reset: db-clean
	@echo "Database reset. Will be recreated on next run."

# Testing with real data
test-import:
	@echo "Testing with testlib catalog..."
	INPX_PATH=./sample-data/testlib.inpx BOOKS_DIR=./sample-data ./pushkinlib

# Check if Docker is available
check-docker:
	@docker --version >/dev/null 2>&1 || (echo "Docker is not installed or not running" && exit 1)
	@docker-compose --version >/dev/null 2>&1 || (echo "Docker Compose is not installed" && exit 1)

# Show status
status:
	@echo "=== Pushkinlib Status ==="
	@echo "Docker containers:"
	@docker ps --filter "name=pushkinlib" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || echo "No containers running"
	@echo ""
	@echo "Volumes:"
	@docker volume ls --filter "name=pushkinlib" 2>/dev/null || echo "No volumes found"
	@echo ""
	@echo "Local files:"
	@ls -la sample-data/*.inpx 2>/dev/null || echo "No INPX files found"
