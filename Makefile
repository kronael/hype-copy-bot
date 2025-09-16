# Default target
.DEFAULT_GOAL := help

.PHONY: build test clean run docker-build docker-run help

# Binary name
BINARY_NAME = main

# Docker image settings
DOCKER_IMAGE = hyperliquid-trade-bot
DOCKER_TAG = latest

# Build the binary
build:
	go build -o $(BINARY_NAME) .

# Run the application
run: build
	./$(BINARY_NAME)

# Run all tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)

# Docker targets
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

# Run in Docker (testnet mode)
docker-run: docker-build
	docker run -it --rm \
		-e HYPERLIQUID_TARGET_ACCOUNT=${HYPERLIQUID_TARGET_ACCOUNT} \
		-e HYPERLIQUID_API_KEY=${HYPERLIQUID_API_KEY} \
		-e HYPERLIQUID_PRIVATE_KEY=${HYPERLIQUID_PRIVATE_KEY} \
		-e HYPERLIQUID_USE_TESTNET=true \
		-e HYPERLIQUID_COPY_THRESHOLD=${HYPERLIQUID_COPY_THRESHOLD:-1000.0} \
		-v $(PWD)/data:/app/data \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Help target
help:
	@echo "Hyperliquid Trade Following Bot"
	@echo ""
	@echo "Commands:"
	@echo "  build         - Build binary"
	@echo "  run           - Build and run locally"
	@echo "  test          - Run all tests"
	@echo "  clean         - Remove build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run in Docker (testnet)"
	@echo ""
	@echo "Environment Variables:"
	@echo "  HYPERLIQUID_TARGET_ACCOUNT - Account to follow (required)"
	@echo "  HYPERLIQUID_API_KEY        - API key (required)"
	@echo "  HYPERLIQUID_PRIVATE_KEY    - Private key (required)"
	@echo "  HYPERLIQUID_USE_TESTNET    - Use testnet (default: false)"
	@echo "  HYPERLIQUID_COPY_THRESHOLD - Minimum trade value (default: 0.01)"
