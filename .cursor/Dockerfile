# Cloud Agent base image for Lumilio Photos.
#
# Stable system dependencies and toolchains live here; repository-dependent
# work (Go modules, frontend deps, dev config, git hooks) is handled by the
# install script after checkout. See CONTRIBUTING.md "Prerequisites".
#
# The Cursor Cloud Agent checks out the repository separately, so this image
# never copies application source.
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Native media libraries (govips -> libvips, libraw, libheif), the embedded
# SQLite headers used by the cgo vector extension, cgo build tooling, and the
# ExifTool / FFmpeg command-line binaries the Server shells out to.
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        build-essential \
        pkg-config \
        libvips-dev \
        libraw-dev \
        libexif-dev \
        libheif-dev \
        libsqlite3-dev \
        ffmpeg \
        libimage-exiftool-perl \
    && rm -rf /var/lib/apt/lists/*

# Go toolchain. Pinned to match `server/go.mod` (`go 1.25.12`) so the toolchain
# directive does not trigger an on-demand download during install.
ARG GO_VERSION=1.25.12
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

# Task runner (repository workflow entry point; CONTRIBUTING.md requires v3.52+).
ARG TASK_VERSION=v3.53.1
RUN curl -fsSL https://taskfile.dev/install.sh \
        | sh -s -- -d -b /usr/local/bin "${TASK_VERSION}"

# Ubuntu 24.04 ships a default non-root `ubuntu` user (uid 1000); the Cloud
# Agent runs as it (see environment.json "user").
USER ubuntu
WORKDIR /home/ubuntu

# Vite+ global CLI (`vp`). It manages the Node.js runtime and package manager
# for the web/ and site/ workspaces. Installed per-user under $HOME.
RUN curl -fsSL https://vite.plus | bash

# Make the Go toolchain, user-level Go bin (swag), and the Vite+ CLI available
# to non-login shells that run install/terminals commands.
ENV PATH="/usr/local/go/bin:/home/ubuntu/go/bin:/home/ubuntu/.local/share/vite-plus/bin:${PATH}"
