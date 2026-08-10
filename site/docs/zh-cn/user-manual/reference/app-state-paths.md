---
title: "应用私有状态路径"
description: "区分 Desktop 用户配置目录和 Docker `/data/app-state` 中的非媒体数据。"
page_id: "reference/app-state-paths"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- desktop/internal/platform/paths.go
- desktop/internal/runtime
- deploy/compose/compose.yml
- server/config/examples/docker/http.toml
-->

# 应用私有状态路径

## Desktop

Desktop 以操作系统的用户配置目录为基础创建 `Lumilio Photos`，并使用以下子目录或文件：

- `settings.v1.json`；
- `storage-shortcuts.v1.json`；
- `secrets/`；
- `logs/`；
- `runtime/`；
- `resources/`；
- `lumen/`；
- `updates/`。

Server 运行时清单还会把数据库、日志、云状态、备份和密钥放入应用状态树。

## Docker

官方 Compose 把宿主机 `LUMILIO_STATE` 路径挂载为 `/data/app-state`。它与 `/data/storage` 必须分开理解和备份。

应用私有状态包含秘密和账户数据。不要把整个目录附加到公开 Issue，也不要与公开媒体共享目录使用相同访问权限。
