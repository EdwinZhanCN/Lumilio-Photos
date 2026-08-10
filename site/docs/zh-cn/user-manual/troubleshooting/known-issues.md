---
title: "当前已知问题与行为差异"
description: "列出本轮代码审查中会直接影响用户决策的已验证限制。"
page_id: "troubleshooting/known-issues"
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
- server/internal/service/auth_passkeys.go
- server/internal/api/dto/passkey_dto.go
- server/internal/service/credential_policy.go
- web/src/features/auth/model/credentialPolicy.ts
- deploy/compose/compose.yml
- server/internal/api/handler/backup_handler.go
- server/internal/service/backup_service.go
- server/internal/api/handler/share_link_handler.go
-->

# 当前已知问题与行为差异

以下内容来自当前 baseline，不是未来承诺：

- 初始化后公开注册仍可用；后续注册账户为普通用户，但没有邀请码或关闭开关。
- 数据库备份下载只返回 SQLite；正式恢复要求配套 manifest，当前没有备份上传或导入入口。
- 公开分享的原件策略执行不一致：ZIP 下载忽略“包含原件”，Web 视频/音频在派生缺失时会回退原件。
- 普通“移入回收站”只软删除数据库记录，原件不移动；没有已验证的永久清空入口。
- 重复项合并显示的可回收字节不会实际释放磁盘。
- 扫描可能重新发现仍在原路径的软删除媒体。
- 资源库公开 API 的重命名仍未提供；“从流明集中移除”只注销目录和私有状态，保留磁盘文件。
- Web 可以向已连接的 Desktop 发起本机目录任务，也可以在 Docker 中打开默认存储位置下的已验证一级候选；它不能提交任意宿主机路径。
- 当前云提供方只有 iCloud，且不应视作完整双向同步。
- macOS Desktop 只发布 Apple Silicon；没有 Linux Desktop 发布产物。
- 默认 Docker Compose 使用 HTTP、主机网络和可变 `latest` 镜像标签。
- 注册 DTO/OpenAPI 仍发布旧的用户名、密码长度约束；Web 与 Server 各自维护策略，当前表单基本对齐但存在再次漂移的风险。

每一项的证据、影响和建议在交付包 `review/product-issues.md` 中单独记录。
