# Use the official Golang image to create a build artifact.
FROM golang:1.23-alpine AS builder

# Create and change to the app directory.
WORKDIR /app

# Copy go.mod and go.sum, download dependencies
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy source code
COPY server/ ./

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker/main.go

# Use a minimal alpine image for the final stage
FROM alpine:3.18

# Install dependencies required for production
RUN apk --no-cache add ca-certificates tzdata exiftool && \
    update-ca-certificates

# Set up directories
RUN mkdir -p /app/data/photos /app/staging /app/queue

# Create a non-root user and group
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy binary from builder
COPY --from=builder /app/worker /app/worker

# Copy necessary config files if any
# COPY config /app/config

# Set working directory
WORKDIR /app

# Change ownership to non-root user
RUN chown -R appuser:appgroup /app

# Use the non-root user to run the container
USER appuser

# Set environment variables
ENV STORAGE_PATH=/app/data/photos \
    STAGING_PATH=/app/staging \
    QUEUE_DIR=/app/queue

# Run the binary
CMD ["/app/worker"]
