---
title: Lumen Intelligence in depth
description: Understand node discovery, connectivity, readiness, and Docker networking boundaries, and troubleshoot by data path.
---

# Lumen Intelligence in depth

For normal deployment start with [Lumen Intelligence](../features/lumen-intelligence). This page only explains networking boundaries and troubleshooting; it is not another installation guide.

## Discovery and inference are two separate links

~~~text
Lumen Intelligence node ready
      │
      ├── mDNS advertises the node address and capabilities ──→ Lumilio Photos
      │
      └── gRPC :50051 ←────────────── Lumilio sends inference tasks directly
~~~

mDNS only tells Photos “which nodes exist on the LAN”. The actual media-derived data and inference results travel directly between the Lumilio Server and the node over gRPC; discovery never proxies them.

When troubleshooting, check each of these separately:

1. Did the node finish downloading and warming up models?
2. Did Photos receive the node's mDNS advertisement?
3. Can Photos reach the advertised address and `50051` directly?
4. Does the node provide the model capability the current task needs?

## The roles of the three programs

| Name | Role | Distribution path |
| --- | --- | --- |
| **Lumen Hub** | Provides Image Semantic Analysis, Person Recognition, OCR Text Recognition, and BioCLIP Species Recognition | Docker images or the native program downloaded by the CLI |
| **lumen-cli** | Detects the platform, generates configuration, downloads and supervises the Hub | Any Windows, macOS, or Linux compute device |
| **Lumen SDK** | Used by Lumilio Photos: discovers nodes, connects, and routes tasks | Bundled with Lumilio Photos; no separate install |

Desktop already embeds the lumen-cli install and supervision flow in its Control Panel. Linux servers, NAS devices, and dedicated GPU hosts use Docker Compose.

## The official Docker networking contract

The published Lumilio Server and Lumen Hub Compose files both use Linux `network_mode: host`. Containers reuse the host's LAN interface directly, so mDNS and gRPC do not go through Docker port translation:

~~~text
LAN
├── NAS host network
│   └── Lumilio Photos container (host network)
├── GPU host network
│   └── Lumen Hub container (host network)
└── Temporarily open PC
    └── lumen-cli / Desktop-managed Lumen Hub
~~~

These three node kinds can coexist and can come online or go offline at any time. Photos connects and routes tasks to capabilities that are already discovered and ready; media storage does not need to move to the compute device.

### Why the release files do not ship a bridge variant

`ports: 50051:50051` only publishes a TCP port; it does not give a bridge container automatic LAN mDNS. Even with a static address written in, you only get one fixed connection and lose the product behavior of nodes joining, leaving, and being discovered without configuration.

So the current releases do not provide bridge networking, extra port mappings, or a Host Broker variant. If the container platform cannot create a host-network project, treat the current Docker distribution path as unsupported on that platform instead of modifying the release Compose to guess at compatibility.

## Startup, health, and discovery order

The Hub starts its control plane first, then downloads and loads models:

1. **starting**: the gRPC process is up, but models are still downloading, loading, or warming up;
2. **healthy**: the standard gRPC Health service reports `Serving`, and the selected services can accept tasks;
3. **mDNS advertisement**: only after readiness does the node publish its address and actual capabilities;
4. **Photos connects**: the SDK receives the node, connects, and shows the capabilities on the status page.

First-start time depends mainly on the models selected in the wizard, the download region, disk, and compute-backend initialization. `lumen-models` is a named volume; rebuilding or upgrading the container reuses already-downloaded models.

The container ships a health check. To verify the current Hub directly:

~~~bash
docker exec lumen-hub lumen-hub healthcheck
~~~

A successful exit means gRPC Health is `Serving`; a reachable TCP port is not the same as models being ready.

## Troubleshoot by symptom

| Symptom | Check first |
| --- | --- |
| Container stays `starting` | `docker logs lumen-hub` for model download, disk space, and backend initialization |
| Vulkan logs show CPU or `llvmpipe` | Does the host have `/dev/dri`? Does the driver support Vulkan 1.3? Is the device mapping still in the Compose file? |
| CUDA container fails to start | Host `nvidia-smi`, NVIDIA Container Toolkit, and whether `gpus: all` is still in the Compose file |
| Hub is `healthy` but Photos has no node | Are both sides using host networking? Does the host firewall and LAN allow mDNS? |
| Photos sees the node but cannot connect | From the Photos host, test TCP `50051` on the advertised address; check the Hub host firewall |
| Node connected but a task is missing | Does the Hub configuration enable the service? Does the status page show that model capability? |
| Node and capabilities fine but the queue does not move | Task switches under Web **Settings → AI**, queue failure records, and repository scope |

To verify basic networking from the Lumilio Photos host, test the Hub's LAN address:

~~~bash
nc -vz 192.168.1.30 50051
~~~

This only confirms TCP reachability; the final authority is the Hub health check and the capabilities shown on the Lumilio status page.

## lumen-cli maintenance commands

The CLI path suits compute devices you can operate directly:

~~~bash
lumen-cli validate   # validate the full configuration
lumen-cli start      # start in the background
lumen-cli run        # run in the foreground and watch logs
lumen-cli reload     # re-read configuration and restart
lumen-cli stop       # stop the background Hub
~~~

The CLI keeps configuration and startup information in `.lumen/` under your home directory, and model caches under `.lumen/models`. Do not copy a Hub archive downloaded for another platform; let the CLI choose the release matching your system and compute backend.

## Manual static nodes are only for explicit advanced topologies

The Lumen SDK still supports `discovery_static_nodes` in the Lumilio Server's full TOML. Use it only when you explicitly accept a fixed node address and maintain the full Server manifest yourself; it can also help temporarily distinguish “mDNS not received” from “gRPC cannot connect”.

Static nodes are not part of the official Docker wizard, and they do not give bridge networking dynamic discovery. Ordinary users should keep the release Compose's host networking and let Photos discover nodes.

## Data boundary

A node receives the media-derived data required for its tasks. Run nodes only on LAN devices you trust, and do not forward `50051` to the public internet. Original media, Lumilio SQLite state, Lumen model caches, and node logs serve different purposes; manage and back them up separately.
