---
title: "连接或运行 Lumen Intelligence 节点"
description: "在 Desktop 或 Docker 场景中建立节点，并从能力状态验证任务。"
page_id: "use/lumen-node"
audience: "管理员、高级用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- desktop/internal/lumen
- deploy/compose/lumen-cpu.compose.yml
- deploy/compose/lumen-cuda.compose.yml
- deploy/compose/lumen-vulkan.compose.yml
- server/internal/service/lumen_service.go
-->

# 连接或运行 Lumen Intelligence 节点

## Desktop 本机节点

Desktop 设置可以安装、配置、启动和查看本机 Lumen Intelligence 状态、缓存与日志。它作为独立受监督子进程运行，不与流明集 Server 混成同一个进程。安装或配置失败时，应查看 Desktop 恢复状态和 Lumen 日志。

## Docker 节点

项目提供 CPU、CUDA 和 Vulkan 叠加配置。CUDA 方案要求 Docker GPU 支持；Vulkan 方案需要把 `/dev/dri` 提供给容器。模型缓存应放在持久化卷中，否则重建容器可能重新下载模型。

## 验证

1. 确认节点进程健康且网络可达。
2. 在流明集设置中刷新能力状态。
3. 检查已发现节点数和活跃节点数。
4. 确认目标任务同时显示“启用”和“可用”。
5. 用少量媒体触发索引并观察任务队列。

<!-- TODO(lumen-network-contract): 节点可以在本机、局域网或其他静态地址，文档不应统一称“本地 AI”。可选术语：本机节点、受控网络节点、远程节点。 -->
