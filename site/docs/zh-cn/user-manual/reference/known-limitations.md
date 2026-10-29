---
title: "当前限制"
description: "集中列出不应由文档暗示已经解决的产品和交付边界。"
page_id: "reference/known-limitations"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "9f10907a42cc5d98ba239aee2888ff38c58f3f83"
last_verified: "2026-10-18"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/internal/storage/storage_assessment.go
- server/internal/cloud/provider_registry.go
- server/internal/api/handler/backup_handler.go
- server/internal/api/handler/share_link_handler.go
- desktop/internal/update/controller.go
- .github/workflows/release.yml
-->

# 当前限制

- 当前没有移动客户端或手机自动备份流程。
- macOS Desktop 只发布 Apple Silicon；没有 Linux Desktop 正式产物。
- Desktop 没有应用内更新；新版本从 GitHub Releases 下载安装。
- 网络存储（NFS、SMB/CIFS、AFP、sshfs、WebDAV、Windows 网络驱动器）上的存储位置和资源库不受支持，并且不能通过确认启用。
- 初始化后公开注册仍存在，没有邀请码或关闭开关。
- 界面下载的数据库快照不包含配套 manifest，且没有正式备份导入端点；恢复只能从服务器的备份目录进行。
- 公开分享的 `include_originals` 没有覆盖 ZIP 下载：只要允许下载，ZIP 中就是原件。
- 安全注销（从流明集中移除）会保留磁盘上的所有文件，物理删除仍不在当前范围内。
- Docker Web 管理面不能登记任意新存储位置，只能使用默认存储位置下已验证的一级候选。
- 回收站保留期在 Docker 中通过 `server.toml` 调整，Desktop 固定为 30 天。
- 云导入目前只有 iCloud，并非完整双向同步。
- 工作室代码证明单份 sidecar，不承诺完整多版本历史。
- Lumen Intelligence 版本兼容矩阵尚未作为公开机器可读产物提供。
- Lumilio Agent 还缺少字段级外部模型数据披露界面。

这些限制在文档中使用明确警告或 TODO 注释处理，不能通过措辞隐藏。每个版本的已知缺陷见[发布说明](./release-notes.md)。
