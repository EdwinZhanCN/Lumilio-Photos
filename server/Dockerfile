# syntax=docker/dockerfile:1

############################################
# Builder
############################################
FROM golang:1.25.0-bookworm AS builder

ENV CGO_ENABLED=1 \
    GO111MODULE=on

# System deps for CGO & libvips (used by govips) & libraw (for modern RAW support)
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

# Build API binary
RUN go build -o /out/server ./cmd

############################################
# Runtime
############################################
FROM debian:bookworm-slim AS runtime

ARG TARGETARCH

# Runtime dependencies: ffmpeg stack, exiftool, libvips runtime, libraw runtime
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        ffmpeg \
        exiftool \
        libvips42 \
        libraw20 \
        tzdata; \
    if [ "${TARGETARCH}" = "amd64" ]; then \
        apt-get install -y --no-install-recommends \
            intel-media-va-driver \
            libva-drm2 \
            libva2; \
    fi; \
    rm -rf /var/lib/apt/lists/*

ENV SERVER_CONFIG_FILE=/app/config/server.local.toml \
    SERVER_PORT=6680 \
    SERVER_ENV=production \
    SERVER_LOG_LEVEL=info \
    LOG_LEVEL=info \
    LOG_DIR=/app/logs \
    LOG_FORMAT_CONSOLE=json \
    LOG_FORMAT_FILE=json \
    DB_HOST=db \
    DB_PORT=5432 \
    DB_USER=postgres \
    DB_PASSWORD=postgres \
    DB_NAME=lumiliophotos \
    DB_SSL=disable \
    LUMILIO_DB_PASSWORD_FILE=/data/storage/.secrets/db_password \
    STORAGE_PATH=/data/storage \
    REPOSITORY_SCAN_ENABLED=true \
    ML_CLIP_ENABLED=true \
    ML_BIOCLIP_ENABLED=true \
    ML_OCR_ENABLED=true \
    ML_FACE_ENABLED=true \
    LUMILIO_SECRET_KEY=/data/storage/.secrets/lumilio_secret_key \
    LUMEN_DISCOVERY_MDNS_ENABLED=false \
    LUMEN_DISCOVERY_HUB_URL=http://host.docker.internal:5866 \
    PATH="/usr/local/bin:${PATH}"

ARG APP_UID=10001
ARG APP_GID=10001

WORKDIR /app

# Create non-root user
RUN groupadd --gid ${APP_GID} app \
    && useradd --uid ${APP_UID} --gid ${APP_GID} --create-home --shell /usr/sbin/nologin app

# App artifacts
COPY --from=builder /out/server /usr/local/bin/server
COPY --from=builder /app/migrations ./migrations
COPY server/config/server.example.toml ./config/server.example.toml
COPY server/config/server.example.toml ./config/server.local.toml

# Prepare storage path
RUN mkdir -p /data/storage/primary && chown -R app:app /data /app

USER app

EXPOSE 6680

HEALTHCHECK --interval=10s --timeout=5s --retries=5 --start-period=20s CMD \
    curl -fsS "http://localhost:${SERVER_PORT}/api/v1/health" >/dev/null || exit 1

ENTRYPOINT ["server"]
