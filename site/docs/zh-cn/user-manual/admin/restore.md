---
title: "恢复数据库"
description: "使用持久恢复操作、重启安装、验证和自动回滚替换当前数据库。"
page_id: "admin/restore"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/db/backup/restore.go
- server/internal/api/dto/backup_dto.go
- server/app/app.go
- web/src/features/settings
-->

# 恢复数据库

恢复前必须先确认对应媒体存储仍存在。数据库恢复不会把原件从快照中还原。

## 操作流程

1. 选择应用列表中的已验证快照。
2. 阅读确认，记录快照名称。
3. 启动恢复；服务先暂存并写入持久操作记录。
4. 运行时重启，页面可能断开。
5. 启动早期安装快照，并先为当前数据库创建 restore point。
6. 验证恢复数据库；成功后继续正常启动，失败则自动回滚。
7. 重新连接页面，检查操作最终状态。

## 恢复后

检查账户、资源库、媒体数量、相册和人工组织；等待 OCR 自动重建，并按需要重建其他派生索引。不要在未验证前删除 restore point 或旧媒体副本。

::: warning 浏览器断开是协议的一部分
不要把 `restart_requested` 阶段的连接断开当作失败。最终判断应以持久恢复操作状态和 Server 就绪检查为准。
:::
