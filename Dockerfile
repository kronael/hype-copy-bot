# Build stage
FROM golang:1.21-bookworm as builder

RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code and Makefile
COPY . .

# Build the binary
RUN make build

# Runtime stage
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main /usr/local/bin/

# Create data directory for potential state persistence
RUN mkdir -p /app/data

# Set default environment variables
ENV HYPERLIQUID_USE_TESTNET=true
ENV HYPERLIQUID_COPY_THRESHOLD=1000.0
ENV HYPERLIQUID_TARGET_ACCOUNT=0xb8b9e3097c8b1dddf9c5ea9d48a7ebeaf09d67d2

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD pgrep main || exit 1

VOLUME ["/app/data"]
