---
title: "状态、队列与日志"
description: "使用健康检查、能力状态、资源库状态和队列样本建立日常观察。"
page_id: "admin/monitor"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/health_handler.go
- server/internal/api/handler/queue_handler.go
- server/internal/storage/directory_manager.go
- web/src/features/monitor
-->

# 状态、队列与日志

Server Monitor 是管理员入口。它应回答四类问题：服务是否存活和就绪、资源库是否在线、依赖能力是否可用、后台任务是否持续前进。

## 日常检查

- live 与 ready 健康状态；
- 当前版本和运行时配置；
- 资源库路径、角色和在线状态；
- 等待、运行、重试和失败任务；
- Lumen 已发现与活跃节点；
- 各智能任务启用与可用状态；
- 磁盘空间与备份时间。

## 日志分层

应用私有日志用于 Server 和宿主运行；每个资源库 `.lumilio/logs` 还预留应用、错误和操作日志。Desktop 有自己的主机和子进程日志。收集时按问题时间截取，不要混合上传全部秘密状态。

反复重启会丢失连续观察上下文。只有在已经保留错误样本，并且重启是恢复步骤的一部分时才执行。
