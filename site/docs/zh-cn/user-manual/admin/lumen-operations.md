---
title: "Lumen Intelligence 运维"
description: "管理节点版本、模型缓存、能力可用性和重建负载。"
page_id: "admin/lumen-operations"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/lumen_service.go
- desktop/internal/lumen
- deploy/compose/lumen-cpu.compose.yml
- server/internal/queue/jobs
-->

# Lumen Intelligence 运维

Lumen Intelligence 节点可以独立于流明集 Server 更新和重启。运维目标是保证任务能力可观测，而不是要求所有节点始终运行。

## 变更前

记录节点版本、能力方案、硬件后端、模型缓存位置和 Server 发现方式。暂停大规模索引，保留当前失败样本。

## 变更后

1. 节点健康。
2. Server 能发现并标记活跃。
3. 每个目标任务显示可用。
4. 使用一个样本完成推理。
5. 逐步恢复索引队列。

## 负载规划

视频图像语义分析、人物识别重建和大批 OCR文字识别会占用计算和存储。不要同时启动所有全库重建；按资源库和任务分批，观察温度、显存、系统内存、模型缓存和队列退避。

Lumen 缓存通常可重建，不应与媒体原件放在同一备份优先级；自定义模型和不可再下载的资源需要单独记录。
