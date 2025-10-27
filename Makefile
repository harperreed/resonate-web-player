# ABOUTME: Makefile for building Resonate Player for different platforms
# ABOUTME: Handles cross-compilation and Docker builds with proper CGO support

.PHONY: all build build-linux build-docker clean run docker-up docker-down help

# Default target
all: build

# Build for current platform
build:
	@echo "Building for current platform..."
	go build -o resonate-player

# Build Linux binary inside Docker (for macOS/Windows hosts)
build-linux:
	@echo "Building Linux binary in Docker container..."
	@echo "Note: This may take a few minutes..."
	docker run --rm \
		-v $(PWD):/workspace \
		-w /workspace \
		-e GOCACHE=/tmp/go-cache \
		-e GO111MODULE=on \
		golang:1.24-bookworm \
		sh -c "apt-get update -qq && apt-get install -y -qq pkg-config libopus-dev libopusfile-dev libasound2-dev > /dev/null 2>&1 && go build -ldflags='-s -w' -gcflags='all=-l' -o resonate-player-linux"
	@mv resonate-player-linux resonate-player
	@echo "✓ Linux binary built successfully"

# Build Docker image using simple Dockerfile
build-docker: build-linux
	@echo "Building Docker image..."
	docker build -f Dockerfile.simple -t resonate-player:latest .
	@echo "Docker image built successfully"

# Build using docker-compose
docker-build: build-linux
	@echo "Building with docker-compose..."
	docker-compose build
	@echo "Build complete"

# Start with docker-compose
docker-up: build-linux
	@echo "Starting resonate-player with docker-compose..."
	docker-compose up -d
	@echo "Container started. View logs with: make docker-logs"

# Stop docker-compose
docker-down:
	@echo "Stopping resonate-player..."
	docker-compose down

# View docker logs
docker-logs:
	docker-compose logs -f

# Run locally
run: build
	@echo "Starting resonate-player locally..."
	./resonate-player

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f resonate-player resonate-player-linux
	@echo "Clean complete"

# Help
help:
	@echo "Resonate Player Build System"
	@echo ""
	@echo "Targets:"
	@echo "  make build         - Build for current platform"
	@echo "  make build-linux   - Build Linux binary (in Docker)"
	@echo "  make build-docker  - Build Docker image"
	@echo "  make docker-up     - Start with docker-compose"
	@echo "  make docker-down   - Stop docker-compose"
	@echo "  make docker-logs   - View docker logs"
	@echo "  make run           - Run locally"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make help          - Show this help"
