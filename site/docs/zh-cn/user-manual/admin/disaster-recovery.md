---
title: "灾难恢复清单"
description: "在主机、状态卷或媒体磁盘损坏后按身份和依赖顺序重建。"
page_id: "admin/disaster-recovery"
audience: "管理员、宿主机操作者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/repository_roots.go
- server/internal/db/backup/restore.go
- server/app/app.go
- server/internal/storage/directory_manager.go
-->

# 灾难恢复清单

## 恢复前

- 冻结损坏现场，避免继续写入；
- 复制可读取的旧媒体和应用私有状态；
- 确认最近数据库快照、媒体备份和应用版本；
- 列出所有 `.lumilioroot` 与 `.lumiliorepo` UUID；
- 准备隔离的新主机或目录。

## 建议顺序

1. 安装兼容版本，但不要创建新的正式图库。
2. 恢复媒体存储和隐藏身份标记到规划路径。
3. 恢复必要配置和加密密钥，保护其权限。
4. 通过正式恢复流程安装数据库快照。
5. 让 Server 重新识别默认和外部存储位置。
6. 处理移动或复制身份冲突。
7. 验证原件和人工图库关系。
8. 最后重建缩略图、转码和智能索引。

如果只剩原件，仍可新建实例并扫描，但账户、相册、人物命名、喜欢、分享和其他数据库状态不能从原件完整推导。
