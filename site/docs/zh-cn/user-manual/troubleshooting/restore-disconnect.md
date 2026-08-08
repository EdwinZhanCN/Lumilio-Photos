---
title: "数据库恢复期间页面断开"
description: "理解恢复操作跨重启继续，以及如何读取最终状态。"
page_id: "troubleshooting/restore-disconnect"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/dto/backup_dto.go
- server/internal/db/backup/restore.go
- server/app/app.go
- web/src/features/settings
-->

# 数据库恢复期间页面断开

数据库恢复不是普通同步请求。服务先验证并暂存快照，写入持久操作记录，随后请求运行时重启；浏览器连接在这个阶段断开是预期行为。

恢复状态依次可能经过：已暂存、请求重启、安装、验证、完成；验证失败时会进入回滚和已回滚，或最终失败。当前数据库在安装前会创建 restore point。

## 该做什么

1. 保留恢复操作 ID 和快照名称。
2. 等待 Server 重新通过就绪检查。
3. 重新打开设置页读取该操作的持久状态。
4. 状态为完成后检查账户、资源库和少量媒体。
5. 状态为已回滚时，原数据库应继续使用；保存错误代码和日志。

不要在“请求重启”阶段反复点击恢复，也不要手工替换正在安装的数据库文件。
