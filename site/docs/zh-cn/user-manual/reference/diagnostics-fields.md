---
title: "诊断字段参考"
description: "列出提交问题时有用、可以脱敏保留的标识和状态。"
page_id: "reference/diagnostics-fields"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/logging/logger.go
- server/internal/api/dto/backup_dto.go
- server/internal/api/handler/capabilities_handler.go
- server/internal/api/handler/queue_handler.go
-->

# 诊断字段参考

## 建议保留

- 应用版本、commit 和构建类型；
- 运行方式、操作系统、架构；
- UTC 时间和本地时区；
- 资源库 UUID、存储位置 UUID 及在线状态；
- 扫描 ID、任务类型、任务 ID、尝试次数和错误代码；
- 恢复操作 ID、快照名称和状态；
- Lumen 已发现节点数、活跃节点数和逐任务可用性；
- HTTP 状态码和请求路径，不包含查询中的秘密令牌。

## 必须删除或替换

- 密码、临时密码、TOTP 种子和恢复代码；
- API Key、Cookie、访问/刷新/媒体令牌；
- 分享令牌；
- 云凭据和会话文件；
- 完整私人路径、用户名、域名和照片内容；
- 数据库和密钥目录。

UUID 通常不直接包含秘密，但可能长期关联你的实例；公开 Issue 中可以稳定替换为 `root-A`、`repo-B`、`job-C`，同时保持引用一致。
