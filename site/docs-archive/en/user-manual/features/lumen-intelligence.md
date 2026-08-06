---
title: Lumen Intelligence
description: Run local-first media understanding on your Desktop, a standalone device, or Docker. Learn about presets, regions, cache, and reconfiguration behavior.
---

<script setup lang="ts">
import DockerComposeConfigurator from "../../../.vitepress/components/DockerComposeConfigurator.vue";
</script>

# Lumen Intelligence

Lumen Intelligence is the media-understanding layer that runs on **your own device or your LAN**: Image Semantic Analysis, Person Recognition, OCR Text Recognition, and BioCLIP Species Recognition. Media and the database stay on the machine that stores them; computation can run on the same machine or on another device on the LAN that is better suited for it.

All of these capabilities are optional. When nothing is configured, enabled, or reachable, importing, browsing, albums, sharing, and backups keep working.

## What these capabilities do for you

| Capability | What you will see | Best for |
| --- | --- | --- |
| Image Semantic Analysis | Find “boats by the sea”, “birthday party”, and similar content in natural language; find similar media from one photo | Finding media by content before you create folders or tags |
| Video semantics | A semantic index built from video frames; search for scenes inside videos | Finding the video that contains a scene |
| Person Recognition | Faces detected, embedded, and clustered; name and organize them on the people page | Reviewing family events or trips by person |
| OCR Text Recognition | Text inside media recognized and indexed for full-text search | Receipts, menus, documents, screenshots, and signs |
| BioCLIP Species Recognition | Species candidates for an image, organized in the nature catalog album | Plants, animals, and nature observations |

These tasks run through background queues on demand. Enabling a task does not process all existing media immediately; check the node, queue, and coverage on the [Server Monitor](./monitor) page, then backfill or rebuild as needed.

## Three supported deployment paths

Choose a path based on two things: **where the media lives** and **where the compute lives**.

| Path | When to use it | Who manages the configuration |
| --- | --- | --- |
| [Desktop, on this machine](#path-1-desktop-managed-local-lumen-intelligence) | You mainly use one Mac or Windows computer | Desktop Control Panel |
| [lumen-cli on a standalone device](#path-2-lumen-cli-managed-standalone-device) | You can operate a Windows, macOS, or Linux machine directly and want to turn it into a LAN node | `lumen-cli configure` |
| [Docker (Linux / NAS)](#path-3-docker-on-linux-nas) | The compute device runs Linux, a NAS, or a dedicated GPU host | Environment variables (thin intent) or published Compose |

All three paths use the same capability interface: Lumilio only sends tasks to nodes that are **discovered and ready**, and it never silently switches to another node when one goes offline.

## Path 1: Desktop-managed local Lumen Intelligence

This is the default Desktop path, for “media and compute on the same computer”.

### First enable

1. Open **Desktop Control Panel → Lumen** and enable the local node.
2. In the configuration wizard, choose the **download region**, **capability preset**, and **compute backend**, and confirm the **model cache location**. See the [unified configuration model](#preset-region-cache-and-profile) for what these mean.
3. Save. Desktop downloads the Lumen Intelligence build that matches your system; the first start also downloads the selected models.
4. Wait until the Control Panel reports **running** or **ready**. The node may start before model download and warmup finish; it cannot process tasks until then.
5. Back in the web app, open the tasks under **Settings → AI**, and confirm node capabilities and queue status on the [Server Monitor](./monitor) page.

In mainland China, downloads and models are fetched through a mirror.

### Reconfiguring after installation

Once installed, you can change the **download region, capability preset, and model cache location** from the Control Panel. Reconfiguration works like this:

1. The Control Panel renders and validates a candidate configuration first; the running node is not stopped until validation passes;
2. Reconfiguring a running node performs a controlled restart: stop → persist the new intent → regenerate and start;
3. If any step fails, the **previous configuration and previous run state** are restored. A half-applied configuration is never left behind.

The installed release profile stays fixed for the current installation; reconfiguration changes the preset, region, and backend rather than replacing the installed program.

The model cache is not part of any repository. Do not put it into media backups, and do not delete cache files manually while the node is running.

## Path 2: lumen-cli-managed standalone device

lumen-cli turns a Windows, macOS Apple Silicon, or Linux computer into a LAN inference node. Use it when you can operate that device's operating system directly.

### Install and configure for the first time

macOS Apple Silicon, Linux x64, or Jetson Linux:

~~~bash
# Download the install script
curl --proto '=https' --tlsv1.2 -LsSf https://lumilio.org/lumen/install.sh | sh

# First-time configuration, guided with arrow keys and Enter
lumen-cli configure

# Start the node; models download automatically afterwards
lumen-cli start
~~~

Windows x64 PowerShell:

~~~powershell
# Download the install script
powershell -ExecutionPolicy Bypass -c "irm https://lumilio.org/lumen/install.ps1 | iex"

# First-time configuration
lumen-cli configure

# Start the node
lumen-cli start
~~~

### configure covers first-time and reconfiguration

`lumen-cli configure` is the one implementation: **use it for both the first configuration and any reconfiguration**. `init` is only a compatibility alias; use `configure` in new environments.

- When reconfiguring, your existing **download region, preset, compute backend, and model cache** appear as interactive defaults; they are only replaced after you confirm;
- After the choices, a summary is shown and a final confirmation is required; if the node is running, it reloads in a controlled way;
- To force a language: `lumen-cli configure --lang en` or `lumen-cli configure --lang zh-CN`; any other value fails explicitly;
- Without `--lang`, the locale is detected in `LC_ALL → LC_MESSAGES → LANG` order; an unsupported high-priority locale falls through instead of blocking.

Configuration is written under `.lumen/` in your home directory (`config.yaml` and `bootstrap.json`). `lumen-cli start` downloads the build matching your system and compute backend, verifies it, and runs it in the background; models download on first start.

### Node information for installation and troubleshooting

Use the following only for installation and diagnostics:

- The node listens on `0.0.0.0:50051` and enables mDNS; allow LAN traffic to TCP `50051` and mDNS multicast in the device firewall.
- Common maintenance commands:

  ~~~bash
  lumen-cli validate   # check the configuration only
  lumen-cli reload     # re-read configuration and restart the node
  lumen-cli run        # run in the foreground and watch logs
  lumen-cli stop       # stop the background node
  ~~~

- If you prefer not to run the install script, download the platform archive and checksums from [Lumen-Hub Releases](https://github.com/EdwinZhanCN/Lumen-Hub/releases). Do not copy an archive for one platform onto another platform.

::: tip Compute backend
macOS Apple Silicon: prefer Metal. Jetson Linux: prefer CUDA. Other Linux ARM platforms: prefer CPU.
:::

## Path 3: Docker on Linux / NAS

Docker is the official distribution path for Linux servers, NAS devices, and dedicated GPU hosts.

### Zero-configuration default: published Compose files

The published `lumen-cpu.compose.yml`, `lumen-vulkan.compose.yml`, and `lumen-cuda.compose.yml` files are complete zero-configuration deployments: they embed the **basic preset**, the **other region**, host networking, and the `lumen-models` persistent volume. No hand-written YAML, `.env` file, or static node address is required.

~~~bash
docker compose -f lumen-cpu.compose.yml up -d --wait
~~~

Before you start, confirm the target device supports modern Docker Compose and Linux `host` networking. Vulkan additionally requires `/dev/dri`; CUDA requires the [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).

### Other regions, presets, or custom combinations: the env + command wizard

To change the download region, capability preset, or model combination, use the wizard below. It outputs only **canonical environment variables (thin intent) and a start command**: Lumen Intelligence renders and validates the complete configuration from those variables at container startup, and the page never generates a complete runtime YAML.

<DockerComposeConfigurator lang="en" />

All three Compose files use Linux host networking; this is the supported boundary for preserving mDNS discovery on the LAN. Do not convert the file to `bridge` networking or add `ports: 50051` and assume discovery remains supported. If the container platform cannot create a host-network project, the current official Docker path does not support discovery on that platform.

### Startup and readiness

The first start downloads and loads the selected models. The container stays `starting` during that time; it becomes `healthy` only when the standard gRPC Health service reports `Serving`, and only then does it start advertising over mDNS.

~~~bash
docker compose -f lumen-cpu.compose.yml ps
docker compose -f lumen-cpu.compose.yml logs -f lumen-hub
~~~

Once the Hub is `healthy`, confirm the node and its model capabilities on the [Server Monitor](./monitor) page. As long as Photos and the node both use host networking from the release files, no extra configuration is needed.

- The **Vulkan** file maps `/dev/dri:/dev/dri`; the **CUDA** file uses `gpus: all`; the **CPU** file is the only one shipping both `amd64` and `arm64` images.

## Preset, region, cache, and profile

The same four concepts apply on every path:

- **Region (download region)**: where programs and models are downloaded from — the official source or the mainland-China mirror. It only affects download routing, not UI language or media location.
- **Preset (capability preset)**: decides which services are enabled, which models are used, and how much space to reserve. When unsure, start with **basic**.

| Preset | Capabilities | Main models and datasets | Resource guidance |
| --- | --- | --- | --- |
| **Minimal** | Image Semantic Analysis, Person Recognition | SigLIP 2 base + InsightFace `antelopev2` | At least 4 GB RAM, 2 GB GPU/unified memory, ~2 GB disk |
| **Basic** | Minimal + OCR Text Recognition + BioCLIP Species Recognition | SigLIP 2 base, PP-OCRv6 small, BioCLIP-2 + `TreeOfLife200MCore` | At least 6 GB RAM, 3 GB GPU/unified memory, ~6 GB disk |
| **Full (brave)** | Stronger Image Semantic Analysis model and full BioCLIP catalog | SigLIP 2 `so400m-patch14-384`, PP-OCRv6 small, BioCLIP-2 + `TreeOfLife200M` | At least 8 GB RAM, 4 GB GPU/unified memory, ~10 GB disk |

The numbers are selection guidance, not hard limits for every device. Model weights and species catalogs are fetched on demand; the first download and warmup can cause short memory peaks.

- **Cache (model cache location)**: where model weights are downloaded. The cache is not part of any repository and should not be included in media backups.
- **Profile (install profile)**: the program shape fixed at Desktop installation and pinned to the current installation transaction. Reconfiguration can change the preset, region, and cache, but cannot swap the pinned profile within the current installation.

### Compute backend

The compute backend decides which hardware runs the models; it does not change Lumilio's task interface:

| Deployment | Available backends | Notes |
| --- | --- | --- |
| Desktop | Metal, GPU, CPU | macOS Apple Silicon: prefer Metal; Windows x64: prefer GPU; CPU is the most compatible |
| lumen-cli | Metal/CPU on macOS; GPU/CPU on Windows; CPU, Vulkan, CUDA, ROCm builds on Linux | `lumen-cli` selects the matching build for the system and hardware |
| Docker | `cpu`, `vulkan`, `cuda` image tags | The backend is decided by the image tag and container device mapping |

Faster backends are usually faster but depend more on drivers, VRAM, or unified memory. On low-power devices, start with the minimal preset and CPU; if a node is discovered but tasks are unavailable, check on the [Server Monitor](./monitor) page whether models finished downloading and warming up.

## Data boundary

A node receives the **media-derived data** required for each task (embeddings, frames, text, and so on); it never receives the repository database or account data. The operator of a remote node can see this data and the node logs, so only use devices you trust. Do not forward port `50051` to the public internet; back up original media, the repository database, model caches, and node logs separately.

For node discovery, connectivity, readiness, and Docker network troubleshooting, see [Lumen Intelligence in depth](../help/lumen-intelligence-details).
