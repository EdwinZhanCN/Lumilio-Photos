# syntax=docker/dockerfile:1

############################################
# Builder
############################################
FROM golang:1.25.0-trixie AS builder

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
FROM debian:trixie-slim AS runtime

ARG TARGETARCH

# Runtime dependencies: ffmpeg stack, exiftool, libvips runtime, libraw runtime.
#
# On trixie, libheif uses a plugin architecture: each codec ships as a separate
# package that --no-install-recommends would otherwise skip. We pin them so image
# encode/decode is guaranteed — libheif-plugin-aomenc (AV1 ENCODE) powers the AVIF
# path of the /assets/{id}/export endpoint; aomdec + libde265 cover reading
# AVIF/HEIC sources. libvips-tools (rather than the versioned libvips42* package)
# lets apt resolve the correct runtime soname across Debian releases.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        ffmpeg \
        exiftool \
        tzdata \
        libvips-tools \
        libheif1 \
        libheif-plugin-aomenc \
        libheif-plugin-aomdec \
        libheif-plugin-libde265 \
        libraw23; \
    if [ "${TARGETARCH}" = "amd64" ]; then \
        apt-get install -y --no-install-recommends \
            intel-media-va-driver \
            libva-drm2 \
            libva2; \
    fi; \
    rm -rf /var/lib/apt/lists/*

# Fail the build loudly if AVIF (AV1) encode is unavailable, rather than silently
# returning HTTP 422 from the export endpoint at runtime. Uses the vips CLI from
# libvips-tools; heifsave defaults to AV1 compression for a .avif target.
RUN set -eux; \
    vips black /tmp/avif-probe.v 16 16; \
    vips heifsave /tmp/avif-probe.v /tmp/avif-probe.avif; \
    test -s /tmp/avif-probe.avif; \
    rm -f /tmp/avif-probe.v /tmp/avif-probe.avif

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
