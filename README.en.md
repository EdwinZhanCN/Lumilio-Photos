<div align="center">

# Lumilio Photos

[简体中文](README.md) | **English**

<img width="128" height="148" alt="Lumilio Photos logo" src="https://github.com/user-attachments/assets/9e51f2dd-af9c-47da-9232-cff9a6e6bf4f" />

Local-first photo and video management for your own Repositories.

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=for-the-badge&logo=react)](https://react.dev/)
[![SQLite](https://img.shields.io/badge/SQLite-3-003B57?style=for-the-badge&logo=sqlite&logoColor=f5f5f5)](https://sqlite.org/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue?style=for-the-badge&logo=gnu)](LICENSE)

</div>

> [!WARNING]
> Lumilio Photos is free, open-source beta software under active development. Back up important Repositories before upgrading and review the release notes for known limitations.

Lumilio Photos keeps your originals and application data on infrastructure you control. It provides one workspace for browsing, importing, organizing, searching, and processing media across Repositories. AI-assisted features are optional: core media management remains usable without a model server or external AI provider.

## Features

- Local-first photo and video management with explicit Repositories
- Albums, people, places, stacks, favorites, and duplicate management
- Upload, folder scanning, metadata extraction, thumbnails, and transcoding
- Search and filters across Repository metadata
- Optional Image Semantic Analysis, Person Recognition, OCR Text Recognition,
  BioCLIP Species Recognition,
  and classification through Lumen
- Responsive web interface plus macOS and Windows desktop packages
- Multi-user authentication with optional MFA and passkeys

## Install

Choose the distribution that matches where Lumilio will run:

| Environment | Recommended method |
| --- | --- |
| macOS (Apple Silicon) | Download the `.dmg` from [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) |
| Windows 10/11 (x64) | Download the `setup.exe` from [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) |
| Linux server or NAS | Use the published Docker Compose images below |
| Contributor workstation | Follow the [contributing guide](CONTRIBUTING.md) |

Desktop packages include the embedded SQLite catalog and required media tools. They run in the system tray or macOS menu bar and open the interface in your default browser at `http://localhost:6680`. See the [installation guide](site/docs/en/user-manual/introduction/installation.md) for platform-specific setup and current signing limitations.

### Docker Compose

The Docker delivery targets Linux with Docker Engine and Compose 2.23.1 or
newer. The default host-network deployment requires no domain or generated
configuration:

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.yml
mkdir -p ./lumilio/media ./lumilio/app-state
docker compose up -d
```

Open `http://<Linux-host-IP>:6680`. Optional
[`caddy.compose.yml`](deploy/compose/caddy.compose.yml) and
[`acme.compose.yml`](deploy/compose/acme.compose.yml) add HTTPS. Read the
[installation guide](site/docs/en/user-manual/introduction/installation.md)
for persistent paths and advanced deployment options. The Compose files use
long bind-mount syntax with `create_host_path: false`, so custom source
directories must exist and a mistyped path fails instead of silently creating
an empty directory.

## Contributing

Development setup, generated-code workflows, tests, and commit conventions live
in [CONTRIBUTING.md](CONTRIBUTING.md).

## Optional AI with Lumen

Image Semantic Analysis, Person Recognition, OCR Text Recognition,
BioCLIP Species Recognition, and
classification are provided by a separate
[Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) inference node. They are
opt-in and are not required for importing, browsing, or organizing media.

- On desktop, use **Enable AI on This Machine** from the tray or menu-bar app. Lumilio Photos downloads and supervises a compatible local Hub.
- For Docker or a remote machine, run Lumen Hub separately and configure node discovery. See [AI and Lumen](site/docs/en/user-manual/introduction/installation.md#optional-ai-features) for setup details.

## Documentation

- [User installation guide](site/docs/en/user-manual/introduction/installation.md)
- [User manual](site/docs/en/user-manual/features/index.md)
- [Contributing](CONTRIBUTING.md)
- [Engineering guide](AGENTS.md)

## License

Lumilio Photos is licensed under the [GNU General Public License v3.0](LICENSE).
