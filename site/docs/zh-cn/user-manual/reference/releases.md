---
title: "版本与发布产物"
description: "理解版本号、发布渠道和每个版本实际构建的产物。"
page_id: "reference/releases"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "9f10907a42cc5d98ba239aee2888ff38c58f3f83"
last_verified: "2026-10-18"
verification_status: "verified"
---

<!--
code-evidence:
- .github/workflows/release.yml
- .github/scripts/release-metadata.sh
- docs/releases
-->

# 版本与发布产物

版本号的格式是 `YY.TRAIN.PATCH[-rc.N | -beta.N]`，例如 `26.1.0-rc.1`。一个 git 标签（`v26.1.0-rc.1`）对应一次发布，同一次发布工作流构建全部产物；任何一个产物失败，GitHub Release 都不会发布。

每个版本的产物：

| 产物 | 平台 | 文件 |
| --- | --- | --- |
| Desktop | macOS Apple Silicon | `Lumilio-Photos-v<版本>-macos-arm64.dmg` |
| Desktop | Windows x64 | `Lumilio-Photos-<版本>-windows-amd64-setup.exe`（安装包）、`Lumilio-Photos-v<版本>-windows-amd64.zip`（便携版） |
| Server | Linux amd64 / arm64 | `ghcr.io/edwinzhancn/lumilio-server:<版本>` 多架构镜像，以及按 digest 固定它的 Compose 包 `Lumilio-Photos-v<版本>-server.tar.gz` |
| 校验 | — | `SHA256SUMS.txt`、`release-manifest.json` |

`rc.N` 和 `beta.N` 发布标记为预发布，不会移动 `latest` 镜像标签。`v26.1.0-rc.1` 是第一个受支持的版本：从它开始，数据库、配置、备份和资源库标记都会被之后的版本原地升级；更早的 beta 数据不会迁移。

每个版本的变更、升级前须知和已知问题见[发布说明](./release-notes.md)；升级步骤见[安全升级](../admin/upgrade.md)。
