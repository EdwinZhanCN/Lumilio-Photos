---
title: "当前已知问题与行为差异"
description: "列出 v26.1.0-rc.1 中会直接影响用户决策的已验证限制。"
page_id: "troubleshooting/known-issues"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "9f10907a42cc5d98ba239aee2888ff38c58f3f83"
last_verified: "2026-10-18"
verification_status: "verified"
---

<!--
code-evidence:
- desktop/main.go
- desktop/internal/update/controller.go
- server/internal/storage/storage_assessment.go
- server/internal/api/router.go
- server/internal/api/handler/share_link_handler.go
- server/internal/api/handler/backup_handler.go
- server/internal/cloud/provider_registry.go
- server/internal/storage/pathsemantics
- server/internal/storage/repo_manager.go
- docs/exec-plans/tech-debt-tracker.md
-->

# 当前已知问题与行为差异

以下内容以 `v26.1.0-rc.1` 为准，不是未来承诺；每个版本的完整列表见[发布说明](../reference/release-notes.md)。

## 行为差异

- **Desktop 构建只在 CI 运行器上经过自动化验证。** rc.1 的 macOS 和 Windows 应用由发布工作流构建，并在 GitHub 托管的 macOS 和 Windows 运行器上通过了 Desktop 自动化测试；发布前没有人在实体机器上启动过它们。请保留独立的媒体备份。
- **不支持网络存储。** NFS、SMB/CIFS、AFP、sshfs、WebDAV 和 Windows 网络驱动器上的存储位置和资源库会被拒绝，并且无法通过确认绕过。流明集运行在 NAS 上、媒体在本地磁盘上不受影响。
- **移入回收站会移动文件。** 文件从你的目录移到资源库自己的回收站，默认 30 天后永久删除；缺失的文件会被隐藏并列在该资源库的“缺失”视图中。详见[重复项与回收站](../use/duplicates-trash.md)。

## 已知问题

- **Desktop 没有应用内更新。** 设置 → 更新 会提示“此构建未配置已签名的更新提供方”。请从 GitHub Releases 下载新版本；应用内更新计划在 rc.2 提供。
- **音频文件内嵌的封面不会显示。** 曲目使用占位图，把音频文件设为相册封面会失败。
- **在不同操作系统之间迁移数据库后，仅大小写不同的重命名可能被当作新文件。** 从 macOS 或 Windows 恢复到 Linux（或反过来）的数据库仍按原系统的规则比较文件名。
- **移除非常大的资源库**会在移除完成前阻塞其他数据库写入。
- **分享相关控件缺少无障碍名称。** 导出对话框中的分享、工作室、添加到相册按钮和公开分享页中的图块不会被屏幕阅读器朗读。
- **分享的 ZIP 下载始终包含原件**：只要允许下载，即使关闭了“包含原件”选项也是如此。
- **初始化之后注册仍然开放。** 任何能打开登录页的人都可以创建普通用户账户，没有邀请码，也没有关闭注册的开关。不要在没有前置访问限制的情况下把实例暴露到公网，见[注册入口暴露](../admin/registration-exposure.md)。
- **备份只能从服务器的备份目录恢复。** 界面下载的 SQLite 快照只是一份副本，没有上传备份文件进行恢复的入口。
- **云导入**只支持 iCloud，并且是单向导入，不是同步。
- **平台。** Desktop 只发布 macOS Apple Silicon 和 Windows x64 版本；Linux 使用 Docker Server；没有移动端应用。
