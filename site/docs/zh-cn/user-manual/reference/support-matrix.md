---
title: "支持的平台与架构"
description: "按实际发布产物列出 Desktop、Server 和容器架构。"
page_id: "reference/support-matrix"
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
-->

# 支持的平台与架构

| 产品面 | 平台 | 架构 | 当前发布形式 |
| --- | --- | --- | --- |
| Desktop | macOS | arm64 / Apple Silicon | DMG |
| Desktop | Windows | amd64 / x64 | 安装包、便携压缩包 |
| Server | Linux Docker | amd64、arm64 | 多架构镜像 |
| Web | 现代浏览器 | 由 Server 提供 | 与 Server 一同交付 |
| Lumen Intelligence | Docker 叠加方案 | 依方案与硬件 | CPU、CUDA、Vulkan 示例 |

当前没有发布 Intel macOS Desktop、Linux Desktop 或移动客户端。源码可以在其他环境编译，不等于项目为其生成、测试和发布正式产物。

Windows Desktop 依赖 WebView2 Runtime。Docker 容器以非 root UID/GID `10001` 运行，并要求媒体和应用私有状态挂载可写。

<!-- TODO(support-policy): 发布工作流没有统一声明最低 macOS/Windows/Linux 内核、浏览器和硬件规格。可选方向：在 release manifest 中生成机器可读支持矩阵。 -->
