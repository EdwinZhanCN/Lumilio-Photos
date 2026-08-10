---
title: "为什么完整备份需要两部分"
description: "用失败场景说明媒体与数据库必须共同保护。"
page_id: "concepts/complete-backup"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/db/backup/backup.go
- server/internal/storage/directory_manager.go
- server/internal/storage/doc.go
- server/internal/api/handler/asset_handler.go
-->

# 为什么完整备份需要两部分

假设只保留数据库：你仍知道有一张照片属于某相册，但原件路径已经空了，无法显示或导出。

假设只保留原件：你可以重新扫描出照片，却失去账户、相册、喜欢、描述、人物命名、事件修正、分享和操作状态。

因此最低完整备份是媒体存储加应用创建的 SQLite 快照。为了重建同一安全环境，还需要受保护的配置和密钥，但它们的泄露风险高于普通照片备份。

派生缩略图、转码和智能索引可以重建，是否备份取决于恢复时间和计算成本；工作室 sidecar 包含人工编辑参数，应随资源库保护。

完整备份的最终证明是恢复演练，而不是文件数量或备份软件的绿色状态。
