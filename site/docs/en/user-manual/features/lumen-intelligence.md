---
title: Lumen Intelligence
description: "Enable Image Semantic Analysis, Person Recognition, OCR Text Recognition, and BioCLIP Species Recognition on Desktop, a standalone device, or Docker."
---

<script setup lang="ts">
import DockerComposeConfigurator from "../../../.vitepress/components/DockerComposeConfigurator.vue";
</script>

# Lumen Intelligence

Lumen Intelligence is the media-understanding layer that runs on **your own device or your LAN**: Image Semantic Analysis, Person Recognition, OCR Text Recognition, and BioCLIP Species Recognition. **Everything is optional** — without it, importing, browsing, albums, sharing, and backups keep working.

| Capability | What you get |
| --- | --- |
| Image Semantic Analysis | Find “boats by the sea” or “birthday party” in natural language, or similar media from one photo |
| Video semantics | Search for scenes inside videos |
| Person Recognition | Faces clustered automatically; name and organize them on the people page |
| OCR Text Recognition | Text in receipts, screenshots, and documents recognized and indexed |
| BioCLIP Species Recognition | Identify plants and animals, organized in the nature catalog album |

Tasks run through background queues on demand; enabling a task does not process existing media immediately. Check coverage on the [Server Monitor](./monitor) page and backfill or rebuild there.

## Which path

Where the media lives and where the compute lives decide the path:

| Path | When to use it |
| --- | --- |
| [Desktop, on this machine](#path-1-desktop-on-this-machine) | You mainly use one Mac/Windows computer |
| [lumen-cli standalone device](#path-2-lumen-cli-standalone-device) | You can operate a Windows/macOS/Linux machine directly |
| [Docker (Linux/NAS)](#path-3-docker-linux-nas) | The compute device is a Linux server, NAS, or GPU host |

All three paths expose the same capabilities: Lumilio only sends tasks to nodes that are **discovered and ready**, and never silently switches when a node goes offline.

## Path 1: Desktop, on this machine

1. Open **Desktop Control Panel → Lumen** and enable the local node;
2. In the wizard choose the **download region**, **capability preset**, and **compute backend**, and confirm the **model cache location** (see [Configuration](#configuration));
3. Save and wait for **running/ready** (tasks cannot be processed until models are downloaded and warmed up);
4. In the web app, open the tasks under **Settings → AI**.

**Reconfiguring**: change region/preset/cache from the Control Panel. The flow is **validate the candidate first → controlled restart while running → restore the previous configuration and run state on failure**. The installed release profile stays fixed for the current installation.

## Path 2: lumen-cli standalone device

macOS Apple Silicon, Linux x64, or Jetson Linux:

~~~bash
curl --proto '=https' --tlsv1.2 -LsSf https://lumilio.org/lumen/install.sh | sh
lumen-cli configure   # use for first-time configuration and reconfiguration
lumen-cli start
~~~

Windows x64 (PowerShell):

~~~powershell
powershell -ExecutionPolicy Bypass -c "irm https://lumilio.org/lumen/install.ps1 | iex"
lumen-cli configure
lumen-cli start
~~~

- **`configure` covers first-time configuration and reconfiguration**; `init` is only a compatibility alias;
- Existing choices are the defaults when reconfiguring; nothing is replaced until you confirm the summary. A running node reloads in a controlled way;
- Force the language with `lumen-cli configure --lang en` or `--lang zh-CN`; without it, the locale is detected in `LC_ALL → LC_MESSAGES → LANG` order and unsupported locales fall through;
- Configuration lives in `~/.lumen/` (`config.yaml`, `bootstrap.json`); `start` downloads the build matching your system and backend, verifies it, and runs it in the background. Models download on first start.

**Install/troubleshooting info**: the node listens on `0.0.0.0:50051` and enables mDNS; allow TCP 50051 and mDNS in the firewall. Commands: `validate` (check config only), `reload` (re-read and restart), `run` (foreground with logs), `stop`.

## Path 3: Docker (Linux/NAS)

**Zero-config default**: the published `lumen-cpu/vulkan/cuda.compose.yml` files embed **basic preset + other region + host networking**:

~~~bash
docker compose -f lumen-cpu.compose.yml up -d --wait
~~~

Requirements: modern Docker Compose and Linux host networking; Vulkan needs `/dev/dri`, CUDA needs the [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).

**Other regions, presets, or custom combinations**: use the wizard below. It outputs only **canonical environment variables (thin intent) + a start command** — the page never generates a complete runtime YAML; Lumen Intelligence renders and validates the full configuration at container startup.

<DockerComposeConfigurator lang="en" />

**Startup and readiness**: the first start downloads models; the container staying `starting` is normal. It becomes `healthy` only when gRPC Health reports `Serving`, and only then advertises over mDNS. Check with `docker compose ps` / `logs -f lumen-hub`. **Do not convert to bridge or add `ports: 50051`** — host networking is the supported boundary for mDNS discovery.

## Configuration

| Item | Meaning | Default / advice |
| --- | --- | --- |
| **region** (download region) | Official source or mainland-China mirror for programs/models | Only affects download routing, not UI language |
| **preset** (capability preset) | Decides which services and models are enabled | Start with **basic** if unsure |
| **cache** (model cache location) | Where model weights are downloaded | Not part of any repository; do not put it in media backups |
| **profile** (install profile) | The program shape fixed at Desktop install | Cannot be swapped in the current installation |

Preset resource guidance (selection aid, not hard limits):

| Preset | Capabilities | Resources |
| --- | --- | --- |
| minimal | Semantic + People | 4 GB RAM / 2 GB GPU / ~2 GB disk |
| basic | All four | 6 GB RAM / 3 GB GPU / ~6 GB disk |
| brave | Stronger semantic model + full species catalog | 8 GB RAM / 4 GB GPU / ~10 GB disk |

**Compute backend**: Desktop picks Metal/GPU/CPU (prefer Metal on Apple Silicon); lumen-cli matches the system automatically; Docker is decided by the image tag (cpu/vulkan/cuda). On low-power devices start with minimal + CPU.

## Data boundary

A node receives only the **media-derived data** required for each task (embeddings, frames, text) — never the database or account data. The operator of a remote node can see this data and its logs, so only use devices you trust; do not expose `50051` to the public internet. Back up original media, the database, model caches, and node logs separately.

Troubleshooting in depth: [Lumen Intelligence in depth](../help/lumen-intelligence-details).
