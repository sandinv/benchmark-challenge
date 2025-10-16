.PHONY: build run test clean install help

# Binary name
BINARY_NAME=benchmark

# Build flags
GOFLAGS=-ldflags="-s -w"

build: ## Build the binary
	@echo "Building..."
	@go build $(GOFLAGS) -o $(BINARY_NAME) .
	@echo "Build complete: $(BINARY_NAME)"

run: build ## Build and run with default settings
	@./$(BINARY_NAME) -inputFile query_params.csv -workers 4

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

clean: ## Remove build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@go clean
	@echo "Clean complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run ./... || true

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

check: fmt vet lint ## Run all checks

db-up: ## Start benchmarking
	@echo "Starting TimescaleDB and benchmark"
	@export POSTGRES_PW=password && docker-compose up -d
	@echo "Waiting for database to be ready..."
	@sleep 5
	@echo "Database ready"

db-down: ## Stop TimescaleDB container
	@echo "Stopping TimescaleDB..."
	@docker-compose down
	@echo "Database stopped"

db-shell: ## Open psql shell
	@docker-compose exec -it timescaledb psql -U postgres -d homework

all: clean deps fmt vet build ## Clean, download deps, format, vet, and build

