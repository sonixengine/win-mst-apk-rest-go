# Multi-stage build for Go Fiber backend
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies (ca-certificates, tzdata)
RUN apk add --no-cache ca-certificates tzdata git

# Copy dependency definition
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build statically compiled binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/master-panel-api ./cmd/server

# Final minimal production image
FROM alpine:3.21

WORKDIR /app

# Install runtime dependencies (ca-certificates, tzdata for Jakarta timezone)
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Jakarta /etc/localtime && \
    echo "Asia/Jakarta" > /etc/timezone

# Create data directory for SQLite persistence
RUN mkdir -p /app/data

# Copy compiled binary from builder stage
COPY --from=builder /app/master-panel-api /app/master-panel-api

# Expose port (default 8080)
EXPOSE 8080

# Run the binary
CMD ["/app/master-panel-api"]
