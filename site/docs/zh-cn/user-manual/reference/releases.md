---
title: "版本与发布产物"
description: "理解当前 baseline 标识、beta 状态和实际构建输出。"
page_id: "reference/releases"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- TAG.txt
- HEAD.txt
- .github/workflows/release-desktop.yml
- .github/workflows/release-docker.yml
-->

# 版本与发布产物

本文档核对的代码标识是：

- 标签描述：`v1.0.0-beta.4-177-g86da6be7`；
- commit：`86da6be7147fa9749c99b914cd79a5f677b92676`；
- 验证日期：2026-08-06。

发布工作流分别构建 macOS Apple Silicon Desktop、Windows x64 Desktop 和 Linux amd64/arm64 Server 镜像。文档中的“当前支持”以这些产物为准，不以某个开发者本机能够编译为准。

beta 版本可能调整配置、数据库 schema、界面术语和资源库生命周期。在升级前阅读目标发布说明、创建数据库快照并保留媒体备份。

<!-- TODO(release-notes): baseline 包未包含按版本整理的用户可读升级说明索引。可选方向：由 change fragments 生成“用户影响、迁移、回退、已知问题”四段式发布说明。 -->
