---
title: "管理与安全"
description: "为存储、账户、网络、备份、升级、队列和恢复建立可操作的管理基线。"
page_id: "admin/index"
audience: "管理员、所有者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/doc.go
- server/internal/db/backup/backup.go
- server/internal/api/router.go
- server/app/app.go
-->

# 管理与安全

管理员负责的不只是“能打开网页”，还包括原件位置、资源库身份、账户入口、数据库快照、媒体备份、升级回退和故障证据。

## 管理顺序

1. 明确[Desktop 或 Server 部署边界](./deployment.md)。
2. 画出[存储位置与资源库](./storage-and-repositories.md)。
3. 关闭不必要的网络暴露并处理[公开注册风险](./registration-exposure.md)。
4. 配置[账户保护](./authentication.md)和[HTTPS](./https.md)。
5. 建立[完整备份](./backup-overview.md)和恢复演练。
6. 在[升级](./upgrade.md)前创建恢复点。
7. 使用[状态、队列和日志](./monitor.md)观察，而不是用反复重启代替诊断。

流明集对原件、数据库和派生文件分层保存。任何“清理”“迁移”或“重置”操作都必须先说明会触碰哪一层。
