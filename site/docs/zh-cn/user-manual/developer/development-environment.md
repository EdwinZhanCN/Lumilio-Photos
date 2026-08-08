---
title: "开发环境"
description: "使用 Taskfile 建立本地状态并启动 Web 与 Server。"
page_id: "developer/development-environment"
audience: "开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- CONTRIBUTING.md
- taskfile.yml
- server/tools/devstate
-->

# 开发环境

仓库声明的主要前置条件包括 Go 1.25+、受支持 Node.js 与 Vite+、Task v3.52+、libvips、libraw、FFmpeg、ExifTool，以及交付验证所需 Docker Compose 2.23.1+。Rust 只在相关组件需要时安装。

```bash
task setup
task dev
```

`task setup` 安装 Server、Web 和文档依赖，生成开发 manifest，安装核心工具和 Git hooks。`task dev` 启动 Web `http://localhost:6657` 和 API `http://localhost:6680`；Web 开发服务器代理 `/api`。

开发运行时数据位于 `.local/dev/`：应用私有状态在 `state/`，可携带开发媒体在 `storage/`。使用 `task dev:clean`、`task dev:reset` 或交互确认的 `task dev:purge` 时，先理解它们对索引、状态和媒体的不同影响。
