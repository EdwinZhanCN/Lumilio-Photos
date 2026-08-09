---
title: "系统要求与支持范围"
description: "依据实际发布流程确认当前可安装的平台、架构和必要运行条件。"
page_id: "getting-started/system-requirements"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- .github/workflows/release-desktop.yml
- .github/workflows/release-docker.yml
- server/Dockerfile
- deploy/compose/compose.yml
-->

# 系统要求与支持范围

当前正式构建产物覆盖以下平台：

| 运行方式 | 当前发布架构 | 关键要求 |
| --- | --- | --- |
| macOS Desktop | Apple Silicon（arm64） | 可运行 DMG 应用；媒体目录可读写 |
| Windows Desktop | x64（amd64） | WebView2 Runtime；媒体目录可读写 |
| Docker Server | Linux amd64、arm64 | Docker；两个可持久化且可写的挂载 |

Windows 构建同时产出安装包和便携压缩包。当前发布工作流没有构建 Intel macOS Desktop，也没有发布 Linux Desktop；不要根据源码可编译性推断官方产物存在。

## 容量规划

媒体原件占用取决于你的现有文件。除此之外，资源库中的 `.lumilio/` 会保存缩略图、转码媒体、人脸裁剪、编辑 sidecar、暂存文件和日志；应用私有状态还包含 SQLite 数据库、备份、云会话和密钥。首次部署前应为这些增长留出空间。

## Lumen Intelligence

基础照片管理可以在没有 Lumen Intelligence 的情况下运行。启用智能能力时，CPU、内存、GPU、模型缓存和任务耗时取决于节点能力及选择的能力方案。当前 Desktop 安装流程公开的 Lumen Intelligence 预设名称是 `minimal`、`basic` 和 `brave`。

<!-- TODO(support-matrix): 代码和发布工作流没有给出统一的最低操作系统版本、最低内存和推荐磁盘空间。可选方向：由真实发布门禁生成支持矩阵，或在发行清单中维护。本文不虚构数值。 -->
