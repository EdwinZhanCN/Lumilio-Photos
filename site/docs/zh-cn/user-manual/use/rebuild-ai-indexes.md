---
title: "重建智能索引"
description: "在节点恢复、设置变化或数据库恢复后，按任务和资源库重建派生索引。"
page_id: "use/rebuild-ai-indexes"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/internal/queue/jobs
- server/app/app.go
- server/internal/service/face_service.go
- server/internal/event
-->

# 重建智能索引

索引重建会重新安排计算任务，不会改写原件。它可能消耗大量 CPU、GPU、磁盘和队列时间，应在基本导入和备份稳定后执行。

## 何时重建

- 新启用图像语义分析、OCR文字识别、人物识别或BioCLIP物种识别能力；
- 节点长期不可用后重新上线；
- 任务失败已经修复；
- 数据库恢复后 OCR 正在自动重建，或其他索引需要补齐；
- 管理员确认索引版本或能力配置发生变化。

## 安全步骤

1. 确认目标资源库在线。
2. 确认节点任务可用。
3. 先在一个资源库或少量范围重建。
4. 在 Server Monitor 观察等待、运行、重试和失败。
5. 不要反复点击重建；唯一性和队列策略可能合并部分任务，但仍会增加负载。

人物和事件重建还会改变派生分组。执行前创建数据库快照，并在完成后检查人工命名、隐藏和成员修正。
