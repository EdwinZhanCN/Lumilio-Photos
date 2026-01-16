<div align="center">

# Lumilio Photos
<img width="128" height="148" alt="logo" src="https://github.com/user-attachments/assets/9e51f2dd-af9c-47da-9232-cff9a6e6bf4f" />

[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=for-the-badge&logo=react)](https://react.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?style=for-the-badge&logo=postgresql&logoColor=f5f5f5)](https://www.postgresql.org/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue?style=for-the-badge&logo=gnu)](LICENSE)

</div>

>[!WARNING]
>🚧 This Project is Under Active Development. 🚧

## Tech Stack

### Web
[![React](https://img.shields.io/badge/React-19.0-61DAFB?logo=react)](https://react.dev/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.8-3178C6?logo=typescript)](https://www.typescriptlang.org/)
[![Vite](https://img.shields.io/badge/Vite-6.1-646CFF?logo=vite&logoColor=f5f5f5)](https://vitejs.dev/)
[![TailwindCSS](https://img.shields.io/badge/TailwindCSS-4.0-06B6D4?logo=tailwind-css)](https://tailwindcss.com/)
[![DaisyUI](https://img.shields.io/badge/DaisyUI-5.1-5A0EF8?logo=daisyui)](https://daisyui.com/)
[![React Query](https://img.shields.io/badge/React_Query-5.74-FF4154?logo=tanstack)](https://tanstack.com/query/latest)
[![Rust](https://img.shields.io/badge/Rust-1.82-000000?logo=rust)](https://www.rust-lang.org/)
[![WebAssembly](https://img.shields.io/badge/WebAssembly-1.0-654FF0?logo=webassembly&logoColor=f5f5f5)](https://webassembly.org/)

### Server
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go)](https://go.dev/)
[![Gin](https://img.shields.io/badge/Gin-1.10-00ADD8?logo=gin)](https://gin-gonic.com/)
[![pgvector](https://img.shields.io/badge/pgvector-0.3-4169E1?logo=postgresql&logoColor=f5f5f5)](https://github.com/pgvector/pgvector)
[![River](https://img.shields.io/badge/River-0.24-5D3F6A?logo=riverqueue)](https://riverqueue.com/)
[![LibRaw](https://img.shields.io/badge/LibRaw-0.20-FF5722?logo=libraw)](https://www.libraw.org/)

## Quick Start

### Production (Docker)

```bash
git clone https://github.com/EdwinZhanCN/Lumilio-Photos.git
cd Lumilio-Photos
docker compose up -d
```

Access:
- Web UI: http://localhost:6657
- API: http://localhost:8080
- API Docs: http://localhost:8080/swagger/index.html

### Development

**Prerequisites:** Go 1.24+, Node.js 20+, Docker, Make

```bash
# Clone and setup
git clone https://github.com/EdwinZhanCN/Lumilio-Photos.git
cd Lumilio-Photos
make setup

# Start everything (database + server + web)
make dev
```

Access:
- Web UI: http://localhost:6657
- API: http://localhost:8080/api/v1/health
- API Docs: http://localhost:8080/swagger/index.html

**Note:** Database runs on port 5433

### Make Commands

```bash
make dev            # Start database, server, and web (recommended)
make server-dev     # Start API server only
make web-dev        # Start web dev server only
make server-test    # Run Go tests
make db-logs        # View database logs
make db-shell       # PostgreSQL shell
make db-reset       # Reset database (⚠️ deletes all data)
make clean          # Clean generated files
```

### Manual Setup

If `make dev` doesn't work for you:

```bash
# Terminal 1: Database
make db-wait

# Terminal 2: Server
cd server
SERVER_ENV=development go run ./cmd

# Terminal 3: Web
cd web
npm run dev -- --host --port 6657
```
