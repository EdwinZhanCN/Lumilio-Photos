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

# Product version stamped into the binary (surfaced via /api/v1/health).
ARG VERSION=dev

# Build API binary
RUN go build -ldflags "-X server/internal/version.Version=${VERSION}" -o /out/server ./cmd
RUN go build -o /out/secretinit ./tools/secretinit

############################################
# Runtime
############################################
FROM debian:trixie-slim AS runtime

ARG TARGETARCH

# Runtime dependencies: ffmpeg stack, exiftool, libvips runtime, libraw runtime,
# PostgreSQL client tools for the automatic database backups.
#
# On trixie, libheif uses a plugin architecture: each codec ships as a separate
# package that --no-install-recommends would otherwise skip. We pin them so image
# encode/decode is guaranteed — libheif-plugin-aomenc (AV1 ENCODE) powers the AVIF
# path of the /assets/{id}/export endpoint; aomdec + libde265 cover reading
# AVIF/HEIC sources. libvips-tools (rather than the versioned libvips42* package)
# lets apt resolve the correct runtime soname across Debian releases.
#
# postgresql-client-18 matches the lumilio-db image. Debian trixie predates
# PostgreSQL 18, so install the client from the official PostgreSQL APT repo.
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends ca-certificates curl; \
    install -d /usr/share/postgresql-common/pgdg; \
    curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc \
        -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc; \
    echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt trixie-pgdg main" \
        > /etc/apt/sources.list.d/pgdg.list; \
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
        postgresql-client-18; \
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
COPY --from=builder /out/secretinit /usr/local/bin/secretinit
COPY --from=builder /app/migrations ./migrations
COPY server/config/server.container.toml ./config/server.toml

# Prepare storage path
RUN mkdir -p /data/storage /data/app-state/cloud /data/app-state/backups /data/app-state/secrets && chown -R app:app /data /app

USER app

EXPOSE 6680

HEALTHCHECK --interval=10s --timeout=5s --retries=5 --start-period=20s CMD \
    curl -fsS "http://localhost:6680/api/v1/health" >/dev/null || exit 1

ENTRYPOINT ["server", "--config", "/app/config/server.toml"]
