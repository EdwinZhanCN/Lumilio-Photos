# syntax=docker/dockerfile:1
#
# Dev container image for Lumilio Photos.
# Includes Go, Node.js, Rust (WASM), and all system-level dependencies
# needed for local development.
#
# NOTE: The db service and other infrastructure containers are defined
# in .devcontainer/docker-compose.yml and run alongside this container.

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
# Use the wasm-pack binary installer instead of `cargo install`
# to avoid compiling it from source (~700MB saved).
# ---------------------------------------------------------------
ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y \
      --default-toolchain nightly-2025-11-15 \
      --profile minimal \
    && rustup component add rust-analyzer \
    && rustup target add wasm32-unknown-unknown \
    && curl -fsSL https://rustwasm.github.io/wasm-pack/installer/init.sh | sh \
    && rm -rf /usr/local/cargo/registry /usr/local/cargo/git

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
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Go CLI tools (river, swag) are installed in postCreateCommand via
# `make setup` so they land on the mounted /root/go volume instead of
# bloating the image layer.

# ---------------------------------------------------------------
# Docker CLI (host socket is mounted at runtime)
# ---------------------------------------------------------------
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian bookworm stable" > /etc/apt/sources.list.d/docker.list && \
    apt-get update && apt-get install -y --no-install-recommends docker-ce-cli && \
    apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

WORKDIR /workspace
