---
title: "升级后无法启动"
description: "保留旧数据和恢复点，从镜像、配置、迁移和依赖逐层回退。"
page_id: "troubleshooting/upgrade-startup"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/app/app.go
- server/internal/db/migration.go
- server/internal/db/backup/restore.go
- server/config
-->

# 升级后无法启动

不要用空数据库或新挂载覆盖旧环境。升级失败时，旧媒体和应用私有状态是恢复依据。

## 检查顺序

1. 确认实际运行的 Desktop 版本或 Docker 镜像标签。
2. 检查严格 TOML 配置是否仍符合当前 schema。
3. 检查两个 Docker 挂载和权限没有变化。
4. 阅读数据库迁移和启动验证错误。
5. 检查外部工具、端口和证书依赖。
6. 若恢复数据库失败，查看是否已经自动回滚到 restore point。

## 回退

使用升级前数据库快照和媒体备份。代码会自动迁移较旧兼容数据库，但拒绝来自未来 schema 或不同 library identity 的不兼容快照。回退应用版本前，先确认旧版本能读取已经迁移的数据库；不能确认时应恢复升级前快照。
