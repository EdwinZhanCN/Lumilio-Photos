---
title: "升级与迁移索引"
description: "按操作类型进入升级、迁移、恢复或回退页面。"
page_id: "reference/upgrade-index"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/app/app.go
- server/internal/db/backup/restore.go
- desktop/internal/update
- deploy/compose/compose.yml
-->

# 升级与迁移索引

| 目标 | 文档 |
| --- | --- |
| 更新 Desktop 或 Docker | [安全升级](../admin/upgrade.md) |
| 升级后无法启动 | [升级后无法启动](../troubleshooting/upgrade-startup.md) |
| 移到新电脑或新主机 | [迁移到新设备](../admin/migrate-device.md) |
| 恢复数据库快照 | [恢复数据库](../admin/restore.md) |
| 退回旧应用或旧目录状态 | [回退](../admin/rollback.md) |
| 主机或磁盘损坏 | [灾难恢复](../admin/disaster-recovery.md) |

任何迁移都应记录源版本、目标版本、数据库快照、媒体备份、存储 UUID 和路径映射。不要把“容器重新创建成功”当作数据迁移完成。
