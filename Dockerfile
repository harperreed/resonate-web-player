# Stage 1: Build stage
FROM golang:1.23-bookworm AS builder

# Install build dependencies for audio libraries
RUN apt-get update && apt-get install -y \
    pkg-config \
    libopus-dev \
    libopusfile-dev \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /build

# Allow Go to automatically download and use newer toolchain versions
ENV GOTOOLCHAIN=auto

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application (dynamic linking for smaller memory usage during build)
RUN CGO_ENABLED=1 GOOS=linux go build -o sendspin-player .

# Stage 2: Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies for audio
RUN apt-get update && apt-get install -y \
    libopus0 \
    libopusfile0 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create app directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/sendspin-player /app/sendspin-player

# Copy default config
COPY config/ /app/config/

# Expose web port
EXPOSE 8080

# Set environment variable for config path
ENV CONFIG_PATH=/app/config/config.yaml

# Run the application
CMD ["/app/sendspin-player"]
