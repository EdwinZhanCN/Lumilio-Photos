---
title: "验证备份与恢复演练"
description: "从文件存在升级为可证明、可重复的恢复能力。"
page_id: "admin/verify-backups"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/db/backup/restore.go
- server/internal/db/backup/backup.go
- server/app/app.go
- server/internal/db/db.go
- server/internal/service/backup_service.go
- server/internal/api/handler/backup_handler.go
-->

# 验证备份与恢复演练

## 每次备份后

- 快照出现在应用列表中；
- 快照进入应用列表，下载文件大小与列表值一致；
- 宿主机备份中同时存在 `.sqlite3` 与对应 `-manifest.json`，并可用 manifest 的 SHA-256 独立校验；
- 媒体备份包含隐藏标记和随机原件；
- 备份保存位置与运行磁盘分离；
- 记录版本、时间、资源库数量和总媒体规模。

## 定期演练

1. 准备隔离测试环境和媒体副本。
2. 使用与快照兼容的流明集版本。
3. 触发正式恢复流程，而不是手工替换 SQLite。
4. 等待操作完成并通过就绪检查。
5. 验证管理员登录、资源库身份、相册、人物命名和少量原件。
6. 记录演练结果和恢复耗时，不把测试环境重新接入生产路径。

恢复流程会拒绝未来 schema 或不同 library identity 的不兼容快照。较旧兼容 schema 可以在下次启动中迁移。
