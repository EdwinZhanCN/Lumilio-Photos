<div align="center">

# Lumilio Photos

[简体中文](README.md) | **English**

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
- Optional semantic search, face recognition, OCR, BioCLIP species recognition,
  and classification through Lumen
- Responsive web interface plus macOS and Windows desktop packages
- Multi-user authentication with optional MFA and passkeys

## Install

Choose the distribution that matches where the library will run:

| Environment | Recommended method |
| --- | --- |
| macOS (Apple Silicon) | Download the `.dmg` from [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) |
| Windows 10/11 (x64) | Download the `setup.exe` from [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) |
| Linux server or NAS | Use the published Docker Compose images below |
| Contributor workstation | Follow the [contributing guide](CONTRIBUTING.md) |

Desktop packages include the embedded SQLite catalog and required media tools. They run in the system tray or macOS menu bar and open the interface in your default browser at `http://localhost:6680`. See the [installation guide](site/docs/en/user-manual/introduction/installation.md) for platform-specific setup and current signing limitations.

### Docker Compose

The Docker delivery targets Linux with Docker Engine and Compose 2.23.1 or
newer. All production variants use host networking:

- [`compose.caddy.yml`](deploy/compose/compose.caddy.yml) — recommended; Caddy
  owns public HTTPS on the same host.
- [`compose.acme.yml`](deploy/compose/compose.acme.yml) — Lumilio obtains and
  terminates the certificate directly.
- [`compose.proxy.yml`](deploy/compose/compose.proxy.yml) — use an existing
  same-host or explicitly configured remote HTTPS proxy.

Read the [installation guide](site/docs/en/user-manual/introduction/installation.md)
before deploying. Production requires an explicit schema-versioned manifest and
separate persistent paths for media and application state.

## Contributing

Development setup, generated-code workflows, tests, and commit conventions live
in [CONTRIBUTING.md](CONTRIBUTING.md).

## Optional AI with Lumen

Semantic embeddings, face recognition, OCR, BioCLIP species recognition, and
classification are provided by a separate
[Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) inference node. They are
opt-in and are not required for importing, browsing, or organizing media.

- On desktop, use **Enable AI on This Machine** from the tray or menu-bar app. Lumilio Photos downloads and supervises a compatible local Hub.
- For Docker or a remote machine, run Lumen Hub separately and configure node discovery. See [AI and Lumen](site/docs/en/user-manual/introduction/installation.md#optional-ai-features) for setup details.

## Documentation

- [User installation guide](site/docs/en/user-manual/introduction/installation.md)
- [User manual](site/docs/en/user-manual/features/index.md)
- [Contributing](CONTRIBUTING.md)
- [Desktop development and packaging](desktop/README.md)
- [Engineering guide](AGENTS.md)

## License

Lumilio Photos is licensed under the [GNU General Public License v3.0](LICENSE).
