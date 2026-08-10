---
title: "队列长期不前进"
description: "用队列摘要、运行任务、重试时间和依赖能力定位停滞。"
page_id: "troubleshooting/queue-stuck"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/internal/api/handler/queue_handler.go
- server/internal/queue
- web/src/features/monitor
-->

# 队列长期不前进

管理员可以在 Server Monitor 查看队列摘要与统计。先确认是真的停滞，而不是任务正在等待退避、资源库离线或节点能力不可用。

## 观察项

- 等待、运行、已计划和失败数量；
- 最旧任务时间；
- 最近错误样本；
- 目标资源库是否在线；
- FFmpeg、ExifTool 或 Lumen 任务是否可用；
- 磁盘空间、数据库写入和系统负载。

队列系统会对部分失败执行重试和退避。反复重启会中断观察窗口，并可能使同一错误不断重新出现。先保留任务 ID、错误、尝试次数和下一次运行时间。

当前管理 API 主要提供只读队列摘要和统计；不要期待文档中不存在的任意“删除全部任务”按钮。
