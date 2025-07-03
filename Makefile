.PHONY: help build test test-integration lint clean docker-build docker-run docker-stop deps fmt vet coverage bench

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build the application
build: ## Build the application
	@echo "Building application..."
	go build -o bin/gokit .

# Install dependencies
deps: ## Install dependencies
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Run tests
test: ## Run unit tests
	@echo "Running unit tests..."
	go test -v -race ./...

# Run integration tests
test-integration: ## Run integration tests (requires Docker)
	@echo "Running integration tests..."
	go test -v -tags=integration -race ./...

# Run all tests
test-all: test test-integration ## Run all tests

# Run tests with coverage
coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Format code
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...
	gofumpt -w .

# Run linter
lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

# Run vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

# Clean build artifacts
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker commands
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t gokit .

docker-run: ## Run with Docker Compose
	@echo "Starting services with Docker Compose..."
	docker-compose up -d

docker-stop: ## Stop Docker Compose services
	@echo "Stopping Docker Compose services..."
	docker-compose down

docker-logs: ## Show Docker Compose logs
	@echo "Showing Docker Compose logs..."
	docker-compose logs -f

# Development setup
dev-setup: deps fmt lint ## Setup development environment
	@echo "Development environment setup complete!"

# Pre-commit checks
pre-commit: fmt lint test ## Run pre-commit checks

# Build for multiple platforms
build-all: ## Build for multiple platforms
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build -o bin/gokit-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build -o bin/gokit-darwin-amd64 .
	GOOS=windows GOARCH=amd64 go build -o bin/gokit-windows-amd64.exe .

# Generate documentation
docs: ## Generate documentation
	@echo "Generating documentation..."
	godoc -http=:6060 &
	@echo "Documentation available at http://localhost:6060"

# Security scan
security: ## Run security scan
	@echo "Running security scan..."
	gosec ./...

# Update dependencies
update-deps: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy 