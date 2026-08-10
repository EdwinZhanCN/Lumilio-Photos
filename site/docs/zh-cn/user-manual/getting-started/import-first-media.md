---
title: "导入第一批媒体"
description: "使用少量副本验证上传或扫描，并确认原件去向。"
page_id: "getting-started/import-first-media"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/manage
- server/internal/api/handler/asset_handler.go
- server/internal/sourcing/materializer.go
- server/internal/storage/scanner/scanner.go
- server/internal/cloud/icloud
-->

# 导入第一批媒体

第一次不要导入全部收藏。准备 10–30 个可替换副本，最好同时包含照片、视频、RAW 或音频中的实际常用类型。

## 选择方式

- **上传**：浏览器把文件写入资源库暂存区，服务计算哈希后提交到 `inbox/` 布局，并登记数据库。
- **扫描**：管理员让流明集发现已经位于资源库可扫描区域中的普通文件。扫描会忽略 `.lumilio/` 和 `inbox/`。
- **云导入**：当前只实现 iCloud 提供方，需要单独连接凭据和启动导入。

个人第一次使用优先选择上传，因为路径最可控。已有大型目录时，先阅读[上传、扫描与云导入](../concepts/upload-scan-cloud.md)，再决定是否把目录放入资源库可扫描区域。

## 验证步骤

1. 在“管理”中确认目标资源库在线。
2. 上传一小批副本。
3. 等待上传状态从等待、上传和处理进入完成；重复文件可能显示为重复。
4. 在资源库中打开媒体，检查缩略图、原件信息和播放。
5. 在磁盘中确认原件最终位置符合所选布局。

失败时不要反复提交同一批大文件。先保留错误信息和任务状态，并转到[上传失败](../troubleshooting/upload-failed.md)。
