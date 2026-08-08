---
title: "备份媒体与资源库工作区"
description: "复制普通原件、身份标记和可恢复 sidecar，同时避免实时写入竞态。"
page_id: "admin/media-backup"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/directory_manager.go
- server/internal/storage/rootcfg/root_config.go
- server/internal/storage/repocfg/repo_config.go
- server/internal/api/handler/asset_handler.go
-->

# 备份媒体与资源库工作区

媒体备份应覆盖默认存储位置和所有外部存储位置。至少包括 `.lumilioroot`、每个 `.lumiliorepo`、普通媒体、`inbox/` 和 `.lumilio/`。

## 建议流程

1. 暂停大规模上传、扫描、转码和编辑保存。
2. 记录所有存储位置和资源库 ID。
3. 使用支持隐藏文件、长路径和元数据的备份工具。
4. 保留版本历史，避免源端误删同步覆盖备份。
5. 随机验证照片、RAW、视频和 sidecar 可读。
6. 保存文件数量、总大小和校验结果。

资源库 `.lumilio/assets` 多数可重建，但 sidecar、暂存失败证据和资源库日志具有恢复或诊断价值。除非备份窗口和容量策略明确，不要默认排除整个 `.lumilio/`。

对于网络或云对象备份，应确认大小写、Unicode、原子重命名和隐藏文件行为不会改变资源库身份文件。
