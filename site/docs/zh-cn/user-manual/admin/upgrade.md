---
title: "安全升级"
description: "在 Desktop 或 Docker 更新前建立恢复点、读取变更并验证关键路径。"
page_id: "admin/upgrade"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-09-03"
verification_status: "verified"
---

<!--
code-evidence:
- server/app/app.go
- server/internal/db/migration.go
- server/cmd/config_cli.go
- desktop/internal/update
- deploy/compose/compose.yml
-->

# 安全升级

升级会自动运行兼容数据库迁移，但这不是跳过备份的理由。配置 schema、容器挂载、Desktop 资源和派生工具也可能变化。

## 升级会如何处理数据

- **数据库**：新版本变更数据库 schema 时，启动过程会先在备份目录写入一份 `pre-upgrade-…` 数据库快照，再升级数据库。保留策略不会删除这份快照，备份列表会把它显示为恢复点。任一升级步骤失败时，Server 不会启动，日志会写明失败的步骤和这份快照。
- **恢复旧备份**：由较早受支持版本创建的备份可以恢复，下次启动时按同样方式升级。
- **Server 配置文件**：Server 不会自行改写 `server.toml`。新版本变更配置格式时，启动会停止并提示显式升级：

  ```bash
  docker compose run --rm lumilio server config upgrade --config /data/app-state/server.toml
  ```

  原文件保存为 `server.toml.bak`，你手动添加的注释只保留在该文件中。默认 Docker 配置内置在镜像中，Desktop 会自动升级自身配置，二者都无需此步骤。
- **降级**：任何版本都会拒绝由更新版本写入的数据库、备份或配置。回退时使用升级前快照。
- **预发布数据**：预发布构建（`v1.0.0-beta.*`、`v26.1.0-beta.*`）的数据不会迁移到 `v26.1.0-rc.1` 及之后的版本。Server 会将这类数据库标明为预发布数据并拒绝打开；请把它移到别处，改用新数据库。原始媒体和资源库不受影响。

## 升级前

1. 阅读目标版本发布说明和已知问题。
2. 创建数据库快照，并在宿主机层把 `.sqlite3` 与配套 manifest 一起复制到独立位置；界面下载的单个 SQLite 只作补充副本。
3. 确认近期媒体备份可读。
4. 记录当前版本、配置、镜像标签、挂载和 Lumen 版本。
5. 等待关键导入和恢复操作完成。

## Desktop

使用与稳定或 beta 更新频道一致的构建。更新后先检查 Desktop 主机、Server 和存储位置状态，再打开全部自动任务。

## Docker

下载并验证目标 Release 的 Server bundle。它通过 `.env` 固定 OCI image digest；升级时保留上一份 bundle、旧 digest 和升级前数据库快照，并确认 Compose、挂载与完整配置仍兼容。不要把 `latest` 的变化当作可审计升级计划。

## 升级后

验证登录、资源库在线、少量媒体播放、队列、数据库快照和可选能力。发现问题时停止新写入并按[回退](./rollback.md)处理。
