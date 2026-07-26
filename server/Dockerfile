# syntax=docker/dockerfile:1

############################################
# Web bundle
############################################
FROM node:24.16.0-trixie-slim AS web-builder

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && corepack enable

WORKDIR /web

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN --mount=type=secret,id=npmrc,target=/root/.npmrc \
    pnpm install --frozen-lockfile

COPY web/. .

ARG VERSION=dev
RUN printf '%s\n' \
    "VITE_API_URL=" \
    "VITE_APP_VERSION=${VERSION}" \
    > .env \
    && pnpm run build

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
    libsqlite3-dev \
    libvips-dev \
    libraw-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app/server

# Go module files first for better caching
COPY server/go.mod server/go.sum ./
COPY server/third_party/river/go.mod ./third_party/river/go.mod
COPY third_party/river-rivershared/go.mod ../third_party/river-rivershared/go.mod
RUN go mod download

# Copy source
COPY server/. .
COPY third_party/river-rivershared ../third_party/river-rivershared

# Product version stamped into the binary (surfaced via /api/v1/health).
ARG VERSION=dev

# Build API binary
RUN go build -tags=sqlite_fts5 -ldflags "-X server/internal/version.Version=${VERSION}" -o /out/server ./cmd

############################################
# Runtime
############################################
FROM debian:trixie-slim AS runtime

ARG TARGETARCH

# Runtime dependencies: ffmpeg stack, exiftool, libvips runtime, libraw runtime,
# curl for health checks, and gosu for dropping privileges after validating
# writable bind mounts.
#
# On trixie, libheif uses a plugin architecture: each codec ships as a separate
# package that --no-install-recommends would otherwise skip. We pin them so image
# encode/decode is guaranteed — libheif-plugin-aomenc (AV1 ENCODE) powers the AVIF
# path of the /assets/{id}/export endpoint; aomdec + libde265 cover reading
# AVIF/HEIC sources. libvips-tools (rather than the versioned libvips42* package)
# lets apt resolve the correct runtime soname across Debian releases.
#
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
        libraw23 \
        gosu; \
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

ENV PATH="/usr/local/bin:${PATH}"

ARG APP_UID=10001
ARG APP_GID=10001

WORKDIR /app

# Create non-root user
RUN groupadd --gid ${APP_GID} app \
    && useradd --uid ${APP_UID} --gid ${APP_GID} --create-home --shell /usr/sbin/nologin app

# App artifacts
COPY --from=builder /out/server /usr/local/bin/server
COPY --from=web-builder /web/dist /app/web
COPY server/config/server.container.toml ./config/server.dev.toml
COPY server/docker-entrypoint.sh /usr/local/bin/lumilio-entrypoint

# Prepare storage path
RUN chmod +x /usr/local/bin/lumilio-entrypoint \
    && mkdir -p /data/storage /data/app-state \
    && chown -R app:app /data /app

EXPOSE 6680 8080 8443

HEALTHCHECK --interval=10s --timeout=5s --retries=5 --start-period=20s CMD \
    server config healthcheck --config /data/app-state/server.toml

ENTRYPOINT ["lumilio-entrypoint"]
CMD ["server", "--config", "/data/app-state/server.toml"]
