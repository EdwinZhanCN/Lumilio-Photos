FROM golang:1.23.3-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY server/go.mod server/go.sum ./

# Download dependencies
RUN cd /app && go mod download

# Copy source code
COPY server/ ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o rkphoto-server ./cmd/main.go

# Create a minimal image for running the application
FROM alpine:latest

WORKDIR /app

# Install necessary packages
RUN apk --no-cache add ca-certificates tzdata

# Copy the binary from builder
COPY --from=builder /app/rkphoto-server .

# Create directory for photos and set permissions
RUN mkdir -p /app/data/photos && \
    chown -R nobody:nobody /app/data/photos && \
    chmod -R 755 /app/data/photos

# Set environment variables
ENV STORAGE_PATH=/app/data/photos

# Expose port
EXPOSE 8080

# Switch to non-root user
USER nobody

# Command to run the application
CMD ["/app/rkphoto-server"]