.PHONY: all build test clean run docker-build docker-run

# Variables
BINARY_NAME=wiki-service
DOCKER_IMAGE=wiki-service
VERSION?=v0.1.0
BIN_DIR=bin

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/wiki-service

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	go test -race ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	rm -f coverage.out coverage.html
	rm -rf wiki-data

# Run the service
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BIN_DIR)/$(BINARY_NAME)

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	podman build -t $(DOCKER_IMAGE):$(VERSION) -f Containerfile .
	podman tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	podman run -d \
		--name wiki-service \
		-p 8080:8080 \
		-v ./wiki-data:/data/wiki:Z \
		-e LLM_ENDPOINT=http://host.containers.internal:8000 \
		$(DOCKER_IMAGE):latest

# Stop and remove Docker container
docker-stop:
	@echo "Stopping Docker container..."
	podman stop wiki-service || true
	podman rm wiki-service || true

# View Docker logs
docker-logs:
	podman logs -f wiki-service

# Initialize development environment
dev-setup:
	@echo "Setting up development environment..."
	go mod download
	mkdir -p wiki-data

# Run all checks (format, lint, test)
check: fmt lint test

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/wiki-service
	GOOS=linux GOARCH=ppc64le go build -o $(BIN_DIR)/$(BINARY_NAME)-linux-ppc64le ./cmd/wiki-service
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/wiki-service
	GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/wiki-service

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-race      - Run tests with race detection"
	@echo "  clean          - Clean build artifacts"
	@echo "  run            - Build and run the service"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  tidy           - Tidy dependencies"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-stop    - Stop Docker container"
	@echo "  docker-logs    - View Docker logs"
	@echo "  dev-setup      - Initialize development environment"
	@echo "  check          - Run format, lint, and test"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  help           - Show this help message"

# Default target
all: clean build test