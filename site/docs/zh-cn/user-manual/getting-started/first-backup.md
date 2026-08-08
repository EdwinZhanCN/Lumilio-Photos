---
title: "创建第一份完整备份"
description: "同时保护媒体存储与 SQLite 数据库快照，并验证两部分可恢复。"
page_id: "getting-started/first-backup"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/db/backup/backup.go
- server/internal/service/settings_service.go
- server/internal/api/dto/settings_dto.go
- server/config/config.go
- server/internal/api/handler/backup_handler.go
- server/internal/service/backup_service.go
-->

# 创建第一份完整备份

流明集的完整备份至少包含两部分：全部媒体存储，以及通过应用创建的 SQLite 数据库快照。任何一部分都不能替代另一部分。

## 第一次备份

1. 在设置中的备份区域创建数据库快照。
2. 确认快照进入已验证列表，记录名称、大小、创建时间、应用版本和 SQLite 版本。
3. 从 Server 的 `backups_path` 复制同名 `.sqlite3` 与对应 `-manifest.json`，并把这一对文件保存到独立故障域。
4. 可以另外使用界面下载 `.sqlite3`，但不要把单个下载文件当作可直接回灌的完整恢复包。
5. 停止或暂停大规模导入，再复制媒体存储，包括普通原件、隐藏标记和 `.lumilio/`。
6. 随机打开备份中的原件，并记录媒体副本与数据库快照的共同时间点。

数据库快照由 SQLite Online Backup API 创建，完成后通过独立连接执行完整性检查，并生成包含 SHA-256、数据库身份和版本信息的 manifest。不要在 Server 正常写入时直接复制实时 SQLite、WAL 和 SHM 文件来代替快照。

::: warning 下载不是完整的导入包
当前下载端点只返回 `.sqlite3`。正式恢复列表只接受 `backups_path` 中同时具有配套 manifest 且大小匹配的快照，当前也没有上传或导入备份的公共端点。因此，离机恢复应保护完整快照配对文件；单独下载的 SQLite 只能作为补充副本。
:::

<!-- TODO(backup-roundtrip): 当前 UI 支持下载单个 SQLite，却没有 manifest 下载和正式备份导入流程。可选方向：导出带签名清单的归档包、提供验证后导入、或明确把整个 backups_path 作为唯一可携带恢复单元。 -->

## 默认自动策略

当前默认设置启用数据库快照，每 24 小时一次，保留最近 14 份；管理员可在 1–720 小时间隔和 1–365 份保留范围内调整。默认快照目录与应用私有状态位于同一挂载，因此它只是本地恢复点，不是独立灾难备份。
