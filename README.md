<div align="center">

# Lumilio Photos

**English** | [简体中文](README.zh-CN.md)

<img width="128" height="148" alt="Lumilio Photos logo" src="https://github.com/user-attachments/assets/9e51f2dd-af9c-47da-9232-cff9a6e6bf4f" />

Local-first photo and video management for your own library.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=for-the-badge&logo=react)](https://react.dev/)
[![SQLite](https://img.shields.io/badge/SQLite-3-003B57?style=for-the-badge&logo=sqlite&logoColor=f5f5f5)](https://sqlite.org/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue?style=for-the-badge&logo=gnu)](LICENSE)

</div>

> [!WARNING]
> Lumilio Photos is free, open-source beta software under active development. Back up important libraries before upgrading and review the release notes for known limitations.

Lumilio Photos keeps your originals and application data on infrastructure you control. It provides one workspace for browsing, importing, organizing, searching, and processing large media libraries. AI-assisted features are optional: the core library remains usable without a model server or external AI provider.

## Features

- Local-first photo and video library with explicit storage repositories
- Albums, people, places, stacks, favorites, and duplicate management
- Upload, folder scanning, metadata extraction, thumbnails, and transcoding
- Search and filters across library metadata
- Optional semantic search, face recognition, OCR, and classification through Lumen
- Responsive web interface plus macOS and Windows desktop packages
- Multi-user authentication with optional MFA and passkeys

## Install

Choose the distribution that matches where the library will run:

| Environment | Recommended method |
| --- | --- |
| macOS (Apple Silicon) | Download the `.dmg` from [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) |
| Windows 10/11 (x64) | Download the `setup.exe` from [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) |
| Linux server or NAS | Use the published Docker Compose images below |
| Contributor workstation | Build from source with `make setup` and `make dev` |

Desktop packages include the embedded SQLite catalog and required media tools. They run in the system tray or macOS menu bar and open the interface in your default browser at `http://localhost:6680`. See the [installation guide](site/docs/en/user-manual/introduction/installation.md) for platform-specific setup and current signing limitations.

### Docker Compose

Docker Engine with Compose 2.23.1+ on Linux is required. Every supported
production stack uses host networking so Lumen discovery and host services have
one explicit network boundary. The single application image serves both the web
interface and API.

The recommended stack runs Caddy on the same host:

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.caddy.yml
export LUMILIO_STORAGE=/srv/lumilio/media
export LUMILIO_STATE=/srv/lumilio/state
export LUMILIO_IMAGE=ghcr.io/edwinzhancn/lumilio-server:latest
export LUMILIO_DOMAIN=photos.example.com
mkdir -p "$LUMILIO_STORAGE" "$LUMILIO_STATE"
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config init --profile docker-external-proxy \
  --origin "https://${LUMILIO_DOMAIN}" --trusted-proxy 127.0.0.1/32 \
  --output /data/app-state/server.toml
docker compose -f compose.caddy.yml up -d
```

Open the exact HTTPS origin you generated. `deploy/compose/compose.acme.yml`
lets Lumilio own public TCP 80/443 directly;
`deploy/compose/compose.proxy.yml` is for an existing same-host or explicitly
configured remote HTTPS proxy. There is no Docker plaintext-development stack;
contributors use `make dev`, while the isolated browser harness owns
`web/e2e/compose.yml`.

> [!IMPORTANT]
> Production reads the complete schema-v3 manifest from `/data/app-state/server.toml`; CLI flags generate or validate that file but never override runtime policy. While Lumilio is running, do not copy or open `library.sqlite3`, `-wal`, or `-shm` with a host SQLite tool. Create consistent snapshots under **Settings → Server**, and back up the media directory separately.

## Development

### Prerequisites

- Go 1.25+
- [Vite+](https://viteplus.dev/) and its supported Node.js runtime
- Make
- Rust and `wasm-pack` for rebuilding browser WASM packages
- Native media libraries and tools: libvips, libraw, FFmpeg, and ExifTool
- Docker with Compose v2 only for container delivery and E2E workflows

Clone and start the development stack:

```bash
git clone https://github.com/EdwinZhanCN/Lumilio-Photos.git
cd Lumilio-Photos
make setup
make dev
```

`make dev` starts the API on `6680` and the web app on `6657`; SQLite runs inside the Go process and needs no database service. `make setup` copies the complete schema-v3 manifest to ignored `server/config/server.local.toml`. The default development catalog is `server/.local/lumilio/library.sqlite3`, while media remains under `server/data/storage`. The server has no config defaults or ordinary environment overrides.

### Useful commands

```bash
make dev              # Start the server and web development processes
make server-dev       # Start only the API server
make web-dev          # Start only the web development server
make test             # Run backend and frontend quality gates
make server-test      # Run Go server tests
make web-test         # Run frontend type, lint, and unit checks
make web-browser-test # Build and run production browser smoke checks
make desktop-test     # Run desktop module tests
make dto              # Regenerate OpenAPI and frontend API types
make db-reset         # Delete development database state (destructive)
make dev-reset        # Recreate local config and SQLite state; preserve media
```

Versioned demo and E2E media comes from the separately released
[`Lumilio-Assets`](https://github.com/EdwinZhanCN/Lumilio-Assets) repository.
From `web/`, run `vp run assets:sync` for the profile pinned in
`assets.lock.json`, or `vp run assets:sync -- --profile=e2e` for another profile
at the same locked revision. Files are hash-verified and stored only in the
ignored `.cache/lumilio-assets/` directory.

## Optional AI with Lumen

Semantic embeddings, face recognition, OCR, and classification are provided by a separate [Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) inference node. They are opt-in and are not required for importing, browsing, or organizing media.

- On desktop, use **Enable AI on This Machine** from the tray or menu-bar app. Lumilio Photos downloads and supervises a compatible local Hub.
- For Docker or a remote machine, run Lumen Hub separately and configure node discovery. See [AI and Lumen](site/docs/en/user-manual/introduction/installation.md#optional-ai-features) for setup details.

## Project Layout

| Path | Purpose |
| --- | --- |
| `server/` | Go API, processing queues, storage, database migrations, and integrations |
| `web/` | React 19 and TypeScript web application |
| `desktop/` | Wails v3 desktop host for the in-process Server and SQLite catalog |
| `wasm/` | Rust WebAssembly packages used by browser-side media workflows |
| `site/` | VitePress user and developer documentation |

## Documentation

- [User installation guide](site/docs/en/user-manual/introduction/installation.md)
- [User manual](site/docs/en/user-manual/features/index.md)
- [Desktop development and packaging](desktop/README.md)
- [Contributor guide](AGENTS.md)

## License

Lumilio Photos is licensed under the [GNU General Public License v3.0](LICENSE).
