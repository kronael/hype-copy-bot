# Default target
.DEFAULT_GOAL := help

.PHONY: build test clean image prepare help

# Binary name
BINARY_NAME = main

# Docker image settings
DOCKER_IMAGE = hype-copy-bot
DOCKER_TAG = latest

# Prepare environment
prepare:
	go mod download
	go mod tidy

# Build the binary
build: prepare
	go build -o $(BINARY_NAME) .

# Run all tests
test:
	go test -timeout=30s .

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Docker image
image:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Help target
help:
	@echo "hype-copy-bot"
	@echo ""
	@echo "Commands:"
	@echo "  prepare       - Download and tidy dependencies"
	@echo "  build         - Build binary"
	@echo "  test          - Run all tests"
	@echo "  clean         - Remove build artifacts"
	@echo "  image         - Build Docker image"
	@echo ""
	@echo "Environment Variables:"
	@echo "  HYPERLIQUID_TARGET_ACCOUNT - Account to follow (required)"
	@echo "  HYPERLIQUID_API_KEY        - API key (required)"
	@echo "  HYPERLIQUID_PRIVATE_KEY    - Private key (required)"
	@echo "  HYPERLIQUID_USE_TESTNET    - Use testnet (default: false)"
	@echo "  HYPERLIQUID_THRESHOLD      - Minimum trade value (default: 0.01)"
