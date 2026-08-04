---
title: Lumen AI
description: Run local media intelligence on a desktop, Linux server, NAS, or a separate LAN compute node.
---

<script setup lang="ts">
import DockerComposeConfigurator from "../../../.vitepress/components/DockerComposeConfigurator.vue";
</script>

# Lumen AI

Lumen Hub provides Image Semantic Analysis, Person Recognition, OCR Text Recognition, and BioCLIP Species Recognition for Lumilio Photos. Storage and compute do not need to share a machine: keep the library and SQLite catalog on a low-power NAS, then run one or more inference nodes on better hardware elsewhere on the LAN.

Once a node is ready, it advertises its address and actual capabilities over mDNS. Lumilio Photos discovers it, connects directly over gRPC, and removes it from the available pool when it goes offline.

## Desktop

On macOS and Windows, open the Desktop Control Panel and choose **Lumen → Enable AI on This Machine**. Desktop selects the native build for the computer, manages its lifecycle, and registers the local node. It also continues to discover other Lumen nodes on the LAN.

The first start downloads the selected models. Wait until the control panel reports that Lumen is ready before expecting AI jobs to run.

## A separate compute machine

If you can access the operating system directly, `lumen-cli` is the simplest way to turn a Windows, macOS Apple Silicon, or Linux machine into a LAN node:

```bash
curl --proto '=https' --tlsv1.2 -LsSf https://lumilio.org/lumen/install.sh | sh
lumen-cli init
lumen-cli start
```

The generated Hub listens on `0.0.0.0:50051` and enables mDNS. Allow LAN traffic to TCP `50051` and mDNS in the host firewall.

## Docker Compose

Docker is the supported path for Linux servers, NAS devices, and dedicated GPU hosts. The wizard embeds the download region, preset, and custom model combination in a complete Compose file; it does not require hand-written YAML, a separate `.env` file, a port mapping, or a static node address.

The target platform must support Linux host networking. Vulkan additionally requires `/dev/dri`; CUDA requires the NVIDIA driver and [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).

<DockerComposeConfigurator lang="en" />

All three files use `network_mode: host`. This is the deployment contract that lets the container advertise directly on the LAN and preserves Lumen's zero-configuration discovery. Do not convert the file to bridge networking or add a `ports` mapping and assume discovery remains supported.

The model cache lives in the named `lumen-models` volume. On first start, the container stays `starting` while models are downloaded and loaded. It becomes `healthy` only when the standard gRPC Health service reports `Serving`; Lumen Hub starts advertising over mDNS at that point.

Open **Server Monitor** in Lumilio Photos after the Hub becomes healthy. The node and its ready model capabilities should appear without editing the Lumilio server manifest.

## Model capabilities

The published Compose defaults to `basic`: SigLIP Base, InsightFace, PP-OCR, and BioCLIP with the Core catalog. The wizard can instead select `minimal`, `brave`, or a validated custom combination. Only lower-level settings outside the wizard require a complete configuration mounted read-only at `/etc/lumen/config.yaml`; remove the Lumen configuration environment variables when that mounted file should remain authoritative.

Lumen sends only the media-derived data required for each inference task to a selected node. Run nodes only on devices you trust, and do not forward port `50051` to the public internet.
