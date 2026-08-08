---
title: "数据库快照"
description: "使用应用内 SQLite Online Backup 和清单验证创建可靠恢复点。"
page_id: "admin/database-snapshots"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/db/backup/backup.go
- server/internal/service/settings_service.go
- server/internal/api/dto/settings_dto.go
- server/internal/service/backup_service.go
- server/internal/api/handler/backup_handler.go
-->

# 数据库快照

流明集通过 SQLite Online Backup API 创建一致快照，而不是直接复制运行中的主数据库文件。快照完成后会从独立连接执行 `quick_check` 和外键检查，并生成包含文件大小、SHA-256、library identity、应用版本、schema 与 SQLite 版本等信息的清单。

## 自动策略

默认启用，每 24 小时一次，保留最近 14 份。管理员可以设置 1–720 小时间隔和 1–365 份保留数；“立即备份”会强制安排新恢复点。

## 默认位置风险

默认备份目录在应用私有状态下，与主 SQLite 通常位于同一宿主挂载。它可用于误操作恢复，但不能抵御整块磁盘、主机或状态卷丢失。应把 `.sqlite3` 与同名 `-manifest.json` 成对复制到独立位置。界面下载只返回 SQLite，不返回 manifest。

## 列表可信度

备份列表只显示命名有效、清单可读且文件大小匹配的条目。一个文件没有出现在列表中不等于可以绕过验证直接恢复。
