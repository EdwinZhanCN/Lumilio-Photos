# syntax=docker/dockerfile:1

############################################
# Builder
############################################
FROM golang:1.24.5-bookworm AS builder

ENV CGO_ENABLED=1 \
    GO111MODULE=on

# System deps for CGO & libvips (used by bimg) & libraw (for modern RAW support)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libvips-dev \
    libraw-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Go module files first for better caching
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy source
COPY server/. .

# CLI used by runtime migrations (river)
RUN go install github.com/riverqueue/river/cmd/river@v0.24.0

# Build API binary
RUN go build -o /out/server ./cmd

############################################
# Runtime
############################################
FROM debian:bookworm-slim AS runtime

# Runtime dependencies: ffmpeg stack, exiftool, libvips runtime, libraw runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    ffmpeg \
    exiftool \
    libvips42 \
    libraw20 \
    tzdata \
    && rm -rf /var/lib/apt/lists/*

ENV STORAGE_PATH=/data/storage/primary \
    SERVER_PORT=8080 \
    SERVER_ENV=production \
    PATH="/usr/local/bin:${PATH}"

ARG APP_UID=10001
ARG APP_GID=10001

WORKDIR /app

# Create non-root user
RUN groupadd --gid ${APP_GID} app \
    && useradd --uid ${APP_UID} --gid ${APP_GID} --create-home --shell /usr/sbin/nologin app

# App artifacts
COPY --from=builder /out/server /usr/local/bin/server
COPY --from=builder /go/bin/river /usr/local/bin/river
COPY --from=builder /app/migrations ./migrations

# Prepare storage path
RUN mkdir -p "${STORAGE_PATH}" /watchman-sock && chown -R app:app /data /app /watchman-sock

USER app

EXPOSE 8080

ENTRYPOINT ["server"]
