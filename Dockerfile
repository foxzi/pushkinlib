# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Download vendor dependencies
RUN apk add --no-cache curl bash && \
    chmod +x ./scripts/download-deps.sh && \
    ./scripts/download-deps.sh && \
    apk del curl bash

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -tags sqlite_fts5 -a -installsuffix cgo -o pushkinlib ./cmd/pushkinlib
RUN CGO_ENABLED=1 GOOS=linux go build -tags sqlite_fts5 -a -installsuffix cgo -o catalog-generator ./cmd/catalog-generator

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    sqlite \
    ca-certificates \
    tzdata

# Create app user
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# Set working directory
WORKDIR /app

# Copy binaries from builder
COPY --from=builder /app/pushkinlib .
COPY --from=builder /app/catalog-generator .

# Copy static files
COPY --from=builder /app/web ./web

# Create directories for data
RUN mkdir -p /data/books /data/cache /data/config && \
    chown -R appuser:appgroup /app /data

# Switch to non-root user
USER appuser

# Environment variables
ENV PORT=9090
ENV BOOKS_DIR=/data/books
ENV CACHE_DIR=/data/cache
ENV DATABASE_PATH=/data/cache/pushkinlib.db
ENV CATALOG_TITLE="Pushkinlib Docker"
ENV LOG_LEVEL=info

# Expose port
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

# Run the application
CMD ["./pushkinlib"]
