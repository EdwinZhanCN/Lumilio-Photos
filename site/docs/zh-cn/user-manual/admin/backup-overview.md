---
title: "备份与恢复总览"
description: "把媒体、数据库快照和应用私有安全材料分成可验证的恢复层。"
page_id: "admin/backup-overview"
audience: "管理员、所有者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/db/backup/backup.go
- server/internal/storage/doc.go
- server/config/schema/lumilio-server.schema.json
- server/internal/storage/directory_manager.go
-->

# 备份与恢复总览

最低完整备份由两部分组成：媒体存储和 SQLite 数据库快照。为了重建同一宿主环境，还应安全保存当前版本、配置、密钥材料和必要云状态，但这些包含敏感信息，不能与公开媒体副本同等处理。

## 备份对象

| 对象 | 包含内容 | 推荐方式 |
| --- | --- | --- |
| 媒体存储 | 原件、标记、`inbox/`、`.lumilio/` | 文件级版本化备份 |
| SQLite 快照 | 账户、相册、人物、事件、标签、任务状态等 | 应用内创建；在宿主机层成对复制 `.sqlite3` 与 manifest |
| 配置与密钥 | Server 清单、加密密钥、证书等 | 加密、受限备份 |
| Lumen 缓存 | 模型和可再生缓存 | 可选；通常可重建 |

## 不能互相替代

数据库快照不包含媒体；媒体副本不包含账户和资源库关系；`.lumilio/` 中的派生文件也不能替代原件。

## 恢复演练

至少定期验证快照可列出、清单哈希匹配、媒体副本可读，并在隔离环境演练数据库恢复。没有演练过的备份只能称为“副本”。
