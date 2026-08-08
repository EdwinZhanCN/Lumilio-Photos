---
title: "当前限制"
description: "集中列出不应由文档暗示已经解决的产品和交付边界。"
page_id: "reference/known-limitations"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/internal/service/asset_service.go
- server/internal/service/duplicate_service.go
- server/internal/cloud/icloud
- server/internal/service/credential_policy.go
- server/internal/api/dto/passkey_dto.go
- server/internal/service/auth_passkeys.go
- web/src/features/auth/model/credentialPolicy.ts
- server/internal/api/handler/backup_handler.go
- server/internal/service/backup_service.go
- server/internal/api/handler/share_link_handler.go
-->

# 当前限制

- 当前没有移动客户端或手机自动备份流程。
- macOS Desktop 只发布 Apple Silicon；没有 Linux Desktop 正式产物。
- 初始化后公开注册仍存在，没有邀请码或关闭开关。
- 界面下载的数据库快照不包含配套 manifest，且没有正式备份导入端点。
- 公开分享的 `include_originals` 没有覆盖 ZIP 下载和 Web 媒体回退路径。
- 资源库重命名与删除公共 API 被禁用。
- Docker Web 管理面不能登记任意新存储根。
- 回收站当前是数据库软删除，未接通物理文件移动和永久清空。
- 重复项“可回收空间”不会实际释放磁盘。
- 扫描可能重新发现原路径仍存在的软删除媒体。
- 云导入目前只有 iCloud，并非完整双向同步。
- 工作室代码证明单份 sidecar，不承诺完整多版本历史。
- Lumen Intelligence 版本兼容矩阵尚未作为公开机器可读产物提供。
- Lumilio Agent 还缺少字段级外部模型数据披露界面。
- 部分 UI 仍混用“仓库”和“资源库”。
- 注册 DTO/OpenAPI 仍发布旧约束；Web 与 Server 的凭据策略没有单一机器可读来源。

这些限制在文档中使用明确警告或 TODO 注释处理，不能通过措辞隐藏。
