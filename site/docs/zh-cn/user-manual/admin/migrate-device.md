---
title: "迁移到新设备或新主机"
description: "同时迁移媒体、数据库、身份和运行配置，并避免创建第二个冲突实例。"
page_id: "admin/migrate-device"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- desktop/internal/platform/paths.go
- server/internal/storage/repo_relocate.go
- server/internal/db/backup/restore.go
- deploy/compose/compose.yml
-->

# 迁移到新设备或新主机

迁移不是只复制照片，也不是只复制数据库。应把它当作受控灾难恢复，在新主机验证完成前保留旧主机只读。

## 步骤

1. 在旧实例停止新导入并创建数据库快照。
2. 备份全部媒体存储、隐藏标记、必要配置和密钥。
3. 在新设备安装兼容版本。
4. 恢复媒体到稳定路径；Docker 同时恢复两个持久挂载。
5. 使用正式恢复流程安装数据库快照。
6. 处理存储位置和资源库的移动身份。
7. 验证后再停用旧实例。

不要让旧、新实例同时对同一可写资源库运行扫描或上传。相同 `.lumiliorepo` 身份被两个实例同时写入会破坏单一所有权假设。

Desktop 默认路径由新操作系统的用户配置目录决定，不要硬编码复制到另一平台的同名绝对路径；使用应用状态和恢复流程映射。
