---
title: "回退应用或数据库"
description: "区分回退运行版本、恢复数据库和恢复媒体三种不同操作。"
page_id: "admin/rollback"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/db/backup/restore.go
- server/internal/db/migration.go
- server/app/app.go
- server/internal/storage/doc.go
-->

# 回退应用或数据库

## 应用版本回退

旧应用未必能读取新版本已经迁移的数据库。只有在兼容性明确时才直接启动旧版本；更安全的方式是同时恢复升级前数据库快照。

## 数据库回退

使用应用内恢复操作安装升级前快照。它会创建当前数据库 restore point，并验证目标快照。数据库回退不会撤销升级期间对原件或资源库文件的手工变更。

## 媒体回退

从版本化媒体备份恢复原件和资源库工作区。不要用较旧媒体树覆盖较新数据库而不检查两者时间关系。

## 验收

确认版本、schema、账户、资源库身份、媒体数量、相册和任务状态。需要重建派生索引时，在核心数据正确后再进行。
