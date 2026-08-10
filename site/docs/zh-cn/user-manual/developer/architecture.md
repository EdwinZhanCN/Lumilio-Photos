---
title: "一页架构概览"
description: "理解 Web、Server、Desktop、SQLite、Lumen Intelligence 和生成契约的边界。"
page_id: "developer/architecture"
audience: "开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/app
- web/ARCHITECTURE.md
- desktop/README.md
- taskfile.yml
- server/tools/architecturecheck
-->

# 一页架构概览

- **Server**：Go 应用，拥有 HTTP API、SQLite、存储、队列、导入、索引、认证和恢复语义。
- **Web**：React/Vite+ 前端，通过生成的 OpenAPI TypeScript 类型访问 Server。
- **Desktop**：Wails v3 宿主，监督内置 Server、SQLite 和可选 Lumen 子进程，并提供系统级目录、网络和更新控制。
- **Lumen Intelligence**：通过协议和锁定发布接入的独立计算系统。
- **WASM**：浏览器端哈希、导出、工作室和缩略图等可选组件。
- **Site**：VitePress 文档与 OpenAPI 文档。

架构约束由 `server/tools/architecturecheck`、前端边界检查和 Taskfile/CI 门禁执行。不要通过跨层导入或手写 DTO 绕过这些约束。
