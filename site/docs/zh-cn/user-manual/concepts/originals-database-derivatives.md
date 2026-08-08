---
title: "原件、数据库与派生文件"
description: "明确哪些数据不可替代、哪些数据可重建，以及修改发生在哪一层。"
page_id: "concepts/originals-database-derivatives"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/directory_manager.go
- server/internal/db/migration.go
- server/internal/api/handler/asset_handler.go
- server/internal/db/backup/backup.go
-->

# 原件、数据库与派生文件

## 原件

原件是用户提供的普通媒体文件。流明集读取它们、登记身份并生成派生内容；工作室编辑不会改写原件。

## 数据库

SQLite 保存资产与路径映射、用户、相册、标签、喜欢、描述、人物命名、事件、分享、任务和设置。数据库不包含全部媒体字节。

## 派生文件

资源库 `.lumilio/assets` 保存缩略图、Web 视频、Web 音频和人脸裁剪；sidecar 保存非破坏编辑；OCR 与语义索引位于应用可重建状态中。

## 恢复含义

只恢复原件可以重新扫描出基础媒体，但不能恢复完整人工组织。只恢复数据库会得到指向缺失文件的记录。完整恢复必须让数据库身份和原件路径重新对应。
