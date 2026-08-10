---
title: "端口与网络参考"
description: "列出默认 HTTP、HTTPS、Lumen 和发现相关网络入口。"
page_id: "reference/ports-network"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/config/examples/docker/http.toml
- deploy/compose/compose.yml
- deploy/compose/caddy.compose.yml
- desktop/internal/runtime
-->

# 端口与网络参考

| 入口 | 默认或示例 | 说明 |
| --- | --- | --- |
| 流明集 HTTP | `6680/tcp` | Desktop 本机/LAN 与默认 Docker HTTP |
| HTTPS | `443/tcp` | Caddy 或内置 TLS |
| ACME HTTP challenge | `80/tcp` | Caddy 或内置 ACME 需要 |
| Lumen gRPC | `50051/tcp` 示例 | Desktop 本机静态节点默认示例 |
| mDNS | 局域网多播 | 可选 Lumen 发现 |

Docker 镜像声明 80、443 和 6680，但实际监听取决于完整配置。官方默认 Compose 使用主机网络，因此端口直接出现在宿主机网络命名空间，不经过普通 `ports` 映射。

Caddy 叠加把流明集上游绑定到 `127.0.0.1:6680`，外部访问由 Caddy 的 80/443 提供。内置 ACME 让流明集自己绑定 80/443。

防火墙只开放实际需要的入口。Lumen gRPC 与 mDNS 不应直接暴露到互联网。
