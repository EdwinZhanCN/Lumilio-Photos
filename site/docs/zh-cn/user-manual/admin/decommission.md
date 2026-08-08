---
title: "停用、导出与保留数据"
description: "在结束实例前证明原件和图库状态已经安全迁出。"
page_id: "admin/decommission"
audience: "管理员、所有者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/share_link_service.go
- server/internal/cloud/sync_service.go
- server/internal/storage/doc.go
- server/config/schema/lumilio-server.schema.json
-->

# 停用、导出与保留数据

停用的完成标准不是容器已经删除，而是你能在不依赖旧实例的情况下读取原件、恢复数据库并解释身份和配置。

## 停用清单

1. 停止新写入。
2. 创建最终数据库快照，并复制 `backups_path` 中的 `.sqlite3` 与配套 manifest；单独的界面下载文件不能替代这一恢复配对。
3. 完整复制全部媒体存储和隐藏标记。
4. 导出必要的配置、版本和挂载清单。
5. 验证备份中的原件和数据库恢复。
6. 撤销公开分享、外部模型 API Key 和云凭据。
7. 停止服务并保留只读观察期。
8. 最后才删除程序、容器、状态卷或旧磁盘。

删除应用私有状态会删除账户和图库关系；删除媒体存储会删除原件。两者都不能通过重新拉取 Docker 镜像恢复。

对于包含密钥、云会话和日志的备份，按敏感数据处理并设置保留期限；不要与公开照片归档混放。
