---
title: "组件与数据兼容性"
description: "说明应用版本、数据库 schema、library identity、Lumen 能力和 Desktop 产物的兼容判断。"
page_id: "reference/compatibility"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/db/backup/restore.go
- server/internal/db/migration.go
- server/internal/api/handler/capabilities_handler.go
- lumen.lock.json
-->

# 组件与数据兼容性

## 数据库

恢复流程拒绝来自未来 schema 或不同 library identity 的快照。较旧兼容 schema 可以安装，并在正常启动中继续迁移。

## 应用回退

数据库迁移通常面向前进，不应假设旧应用能读取新 schema。回退版本时优先配对升级前快照。

## Lumen Intelligence

流明集按任务能力而不是只按“节点在线”判断可用性。节点必须声明并实际提供图像语义分析、OCR文字识别、人物识别或BioCLIP物种识别等具体任务。

## Desktop

当前 macOS 和 Windows 产物架构不同，不能跨平台直接替换程序文件。媒体和数据库可以通过正式迁移流程跨宿主恢复。

<!-- TODO(compatibility-matrix): baseline 未提供公开、机器可读的 Lumilio Photos ↔ Lumen Intelligence 版本矩阵。可选方向：协议握手导出兼容范围并在发布站生成矩阵。 -->
