# Multi-stage build for LogAnalyzer
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o loganalyzer ./cmd

# Final minimal image with Docker CLI support
FROM alpine:latest

# Install Docker CLI (without daemon)
RUN apk add --no-cache docker-cli ca-certificates tzdata

# Copy binary
COPY --from=builder /build/loganalyzer /loganalyzer

# Run as root to access docker socket
# In production, you'd want to use proper user/group mapping

# Expose ports
EXPOSE 9090 9091 8080

# Health check (disabled - needs proper implementation)
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#   CMD ["/loganalyzer", "--health-check"] || exit 1

ENTRYPOINT ["/loganalyzer"]