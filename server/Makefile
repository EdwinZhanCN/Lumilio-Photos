# Makefile for RKPhoto Manager Server

.PHONY: help docs docs-gen docs-serve build test clean run dev

# Default target
help:
	@echo "Available commands:"
	@echo "  docs-gen    Generate Swagger documentation"
	@echo "  docs-serve  Serve Swagger documentation"
	@echo "  docs        Generate and serve documentation"
	@echo "  build       Build the application"
	@echo "  test        Run tests"
	@echo "  run         Run the application"
	@echo "  dev         Run in development mode with hot reload"
	@echo "  clean       Clean build artifacts"

# Install swag CLI if not present
install-swag:
	@which swag > /dev/null || go install github.com/swaggo/swag/cmd/swag@latest

# Generate Swagger documentation
docs-gen: install-swag
	@echo "Generating Swagger documentation..."
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	@echo "Documentation generated in docs/ directory"

# Serve documentation (requires Python3)
docs-serve:
	@echo "Serving documentation at http://localhost:8080"
	@cd docs && python3 -m http.server 8080

# Generate and serve documentation
docs: docs-gen docs-serve

# Build the application
build:
	@echo "Building API server..."
	go build -o bin/api ./cmd/api/main.go
	@echo "Building worker..."
	go build -o bin/worker ./cmd/worker/main.go

# Run tests
test:
	go test -v ./...

# Run the API server
run: build
	./bin/api

# Run in development mode (requires air for hot reload)
dev:
	@which air > /dev/null || go install github.com/cosmtrek/air@latest
	air -c .air.toml

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf docs/docs.go docs/swagger.json docs/swagger.yaml

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

# Run database migrations (if applicable)
migrate:
	@echo "Running database migrations..."
	# Add migration commands here if needed

# Docker build
docker-build:
	docker build -f Dockerfile.api -t rkphoto-manager-api .
	docker build -f Dockerfile.worker -t rkphoto-manager-worker .

# Docker compose up
docker-up:
	docker-compose up -d

# Docker compose down
docker-down:
	docker-compose down

# Full development setup
setup: deps docs-gen
	@echo "Development environment setup complete!"
	@echo "Run 'make dev' to start development server"
	@echo "Run 'make docs-serve' to view API documentation"

# Production build
prod-build: clean deps test lint build docs-gen
	@echo "Production build complete!"
