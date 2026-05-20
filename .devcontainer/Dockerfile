# syntax=docker/dockerfile:1
#
# Dev container image for Lumilio Photos.
# Includes Go, Node.js, Rust (WASM), and all system-level dependencies
# needed for local development.
#
# NOTE: The db service and other infrastructure containers are defined
# in docker-compose.dev.yml and run alongside this container.

FROM golang:1.25.0-bookworm

# ---------------------------------------------------------------
# Node.js 23.x + pnpm (matches web/.nvmrc)
# ---------------------------------------------------------------
RUN curl -fsSL https://deb.nodesource.com/setup_23.x | bash - && \
    apt-get install -y nodejs && \
    npm install -g pnpm@latest && \
    rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------------
# Rust toolchain (nightly + wasm target for WASM crates)
# ---------------------------------------------------------------
ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal && \
    rustup toolchain install nightly-2025-11-15 && \
    rustup target add wasm32-unknown-unknown && \
    cargo install wasm-pack && \
    rm -rf /usr/local/cargo/registry/

# ---------------------------------------------------------------
# System dependencies for Go CGO (libvips, libraw) + media tools
# ---------------------------------------------------------------
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libvips-dev \
    libraw-dev \
    ffmpeg \
    exiftool \
    ca-certificates \
    git \
    unzip \
    curl \
    && rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------------
# Go CLI tools used during development
# ---------------------------------------------------------------
RUN go install github.com/riverqueue/river/cmd/river@v0.24.0 \
    && go install github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5 \
    && rm -rf /root/go/pkg/mod /root/.cache

# ---------------------------------------------------------------
# Docker CLI (so `docker` commands work from inside the container
# via the mounted host socket)
# ---------------------------------------------------------------
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian bookworm stable" > /etc/apt/sources.list.d/docker.list && \
    apt-get update && apt-get install -y --no-install-recommends docker-ce-cli && \
    rm -rf /var/lib/apt/lists/*

# ---------------------------------------------------------------
# Clean up
# ---------------------------------------------------------------
RUN apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /workspace
