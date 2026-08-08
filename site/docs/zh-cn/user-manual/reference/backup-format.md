---
title: "数据库快照格式与恢复状态"
description: "说明快照命名、清单、验证和恢复点。"
page_id: "reference/backup-format"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/db/backup/backup.go
- server/internal/db/backup/restore.go
- server/internal/api/dto/backup_dto.go
- server/internal/api/handler/backup_handler.go
- server/internal/service/backup_service.go
-->

# 数据库快照格式与恢复状态

数据库快照文件名包含 UTC 时间和 library 标识后缀，示例形式为 `20260711T020000.000000Z-library.sqlite3`。对应清单名为 `20260711T020000.000000Z-library-manifest.json`。每个有效快照必须配有清单，至少记录：

- 文件名、大小和 SHA-256；
- 创建时间；
- library identity；
- 应用、配置和迁移版本；
- SQLite 与扩展版本；
- `quick_check` 结果；
- 外键违规计数。

恢复前会重新验证这些信息。安装目标快照前，当前数据库会以 `restore-point-` 前缀创建恢复点。

恢复操作具有跨重启的持久 ID 和状态。状态为 `completed` 才代表新数据库通过验证并继续启动；`rolled_back` 表示恢复安全失败且旧数据库重新启用。

快照不包含媒体、sidecar、派生文件、云会话或密钥。当前下载 API 只返回 SQLite 文件；正式列表与恢复要求服务器备份目录中保留配套 manifest，且没有公共上传端点。
