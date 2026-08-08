---
title: "选择运行方式"
description: "在 Desktop 与 Docker Server 之间选择，并理解两者共同和不同的责任边界。"
page_id: "getting-started/choose-runtime"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- desktop/README.md
- desktop/main.go
- desktop/internal/storage/controller.go
- deploy/compose/compose.yml
- server/internal/api/router.go
-->

# 选择运行方式

Desktop 与 Docker Server 使用同一套流明集 Server 能力，但宿主方式不同。选择时优先考虑谁负责运行、是否需要长期在线，以及媒体位于哪台设备。

| 场景 | 更合适的方式 |
| --- | --- |
| 一个人在 Mac 或 Windows 电脑上管理本地或外置磁盘 | Desktop |
| 家庭成员通过浏览器共同访问 | Docker Server |
| 服务需要持续在线，并由 Linux 主机或 NAS 承载 | Docker Server |
| 希望由桌面界面管理 Server、存储位置和本机 Lumen | Desktop |
| 已有反向代理、备份和容器运维体系 | Docker Server |

## Desktop

Desktop 通过 Wails 承载界面，并在应用内部启动 Server 与 SQLite。它还能管理本机运行时、添加存储位置，并可选安装和监督本机 Lumen Intelligence 子进程。关闭 Desktop 或停止其运行时后，浏览器端服务也会停止。

## Docker Server

Docker 镜像以非 root 用户运行，媒体与应用私有状态必须通过不同挂载保存。默认 Compose 使用主机网络并在 `6680` 端口提供 HTTP 服务。Docker 的 Web 管理面目前只能列出已知存储位置，不能像 Desktop 一样通过目录选择器注册任意新根目录。

::: tip 简化原则
个人电脑优先选择 Desktop；需要多人和长期在线时选择 Docker Server。不要仅因为“以后可能会扩展”而提前承担不必要的容器和网络复杂度。
:::
