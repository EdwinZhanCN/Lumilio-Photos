<div align="center">

# Lumilio Photos

**English** | [简体中文](README.zh-CN.md)

<img width="128" height="148" alt="Lumilio Photos logo" src="https://github.com/user-attachments/assets/9e51f2dd-af9c-47da-9232-cff9a6e6bf4f" />

Local-first photo and video management for your own library.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=for-the-badge&logo=react)](https://react.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-17-4169E1?style=for-the-badge&logo=postgresql&logoColor=f5f5f5)](https://www.postgresql.org/)
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

Desktop packages include a private PostgreSQL runtime and the required media tools. They run in the system tray or macOS menu bar and open the interface in your default browser at `http://localhost:6680`. See the [installation guide](site/docs/en/user-manual/introduction/installation.md) for platform-specific setup and current signing limitations.

### Docker Compose

Docker with the Compose plugin is required. Set `LUMILIO_STORAGE` to the host media directory and `LUMILIO_BOOTSTRAP_PASSWORD_FILE` to a non-empty private file used by both PostgreSQL initialization and the server manifest.

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/docker-compose.release.yml
export LUMILIO_STORAGE=/srv/photos
export LUMILIO_BOOTSTRAP_PASSWORD_FILE=/srv/lumilio-secrets/db_bootstrap_password
# Create the secret with a password manager, or use the image's idempotent helper:
mkdir -p "$(dirname "$LUMILIO_BOOTSTRAP_PASSWORD_FILE")"
docker run --rm --entrypoint secretinit \
  -v "$(dirname "$LUMILIO_BOOTSTRAP_PASSWORD_FILE"):/secrets" \
  ghcr.io/edwinzhancn/lumilio-server:latest /secrets/db_bootstrap_password
docker compose -f docker-compose.release.yml up -d
```

Then open `http://localhost:6657` and complete the first-run wizard. The web UI listens on port `6657` (HTTP) and `6658` (HTTPS); the API listens on `6680`.

To pin a release instead of following `latest`:

```bash
LUMILIO_VERSION=v1.0.0 LUMILIO_STORAGE=/srv/photos \
  docker compose -f docker-compose.release.yml up -d
```

> [!IMPORTANT]
> The complete runtime manifest is baked into the image at `/app/config/server.toml`; ordinary environment variables do not override it. It references the Compose bootstrap secret and creates the app root key beneath `LUMILIO_STORAGE`. Mount a different complete schema v1 manifest at that path to change immutable policy.

## Development

### Prerequisites

- Go 1.25+
- [Vite+](https://viteplus.dev/) and its supported Node.js runtime
- Docker with Compose v2
- Make
- Rust and `wasm-pack` for rebuilding browser WASM packages

Clone and start the development stack:

```bash
git clone https://github.com/EdwinZhanCN/Lumilio-Photos.git
cd Lumilio-Photos
make setup
make dev
```

`make dev` starts PostgreSQL on host port `5433`, the API on `6680`, and the web app on `6657`. `make setup` copies the complete schema v1 manifest to ignored `server/config/server.local.toml` and idempotently creates its bootstrap secret. The server has no config defaults or env overrides; after pulling this breaking change, run `make dev-reset` to discard the incompatible pre-manifest development database/config state.

### Useful commands

```bash
make dev              # Start database, server, and web
make db               # Start the development PostgreSQL service
make server-dev       # Start only the API server
make web-dev          # Start only the web development server
make test             # Run backend and frontend quality gates
make server-test      # Run Go server tests
make web-test         # Run frontend type, lint, and unit checks
make web-browser-test # Build and run production browser smoke checks
make desktop-test     # Run desktop module tests
make dto              # Regenerate OpenAPI and frontend API types
make db-reset         # Delete development database state (destructive)
make dev-reset        # Recreate config, bootstrap secret, and DB state (destructive)
```

The repository also includes a Dev Container configuration under `.devcontainer/`. Open the project in the container, run `make setup`, and then use the same `make dev` workflow.

## Optional AI with Lumen

Semantic embeddings, face recognition, OCR, and classification are provided by a separate [Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) inference node. They are opt-in and are not required for importing, browsing, or organizing media.

- On desktop, use **Enable AI on This Machine** from the tray or menu-bar app. Lumilio Photos downloads and supervises a compatible local Hub.
- For Docker or a remote machine, run Lumen Hub separately and configure node discovery. See [AI and Lumen](site/docs/en/user-manual/introduction/installation.md#optional-ai-features) for setup details.

## Project Layout

| Path | Purpose |
| --- | --- |
| `server/` | Go API, processing queues, storage, database migrations, and integrations |
| `web/` | React 19 and TypeScript web application |
| `desktop/` | Wails v3 desktop host and private PostgreSQL supervisor |
| `wasm/` | Rust WebAssembly packages used by browser-side media workflows |
| `site/` | VitePress user and developer documentation |

## Documentation

- [User installation guide](site/docs/en/user-manual/introduction/installation.md)
- [User manual](site/docs/en/user-manual/features/index.md)
- [Desktop development and packaging](desktop/README.md)
- [Contributor guide](AGENTS.md)

## License

Lumilio Photos is licensed under the [GNU General Public License v3.0](LICENSE).
