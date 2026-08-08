---
title: "Desktop 与 Server 的运行边界"
description: "理解同一产品能力如何由桌面宿主或长期容器承载。"
page_id: "concepts/desktop-and-server"
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
- server/cmd/main.go
- deploy/compose/compose.yml
-->

# Desktop 与 Server 的运行边界

Desktop 内嵌并监督流明集 Server 与 SQLite，同时提供操作系统目录选择、网络模式、更新和本机 Lumen 管理。它适合把宿主机责任收进桌面应用。

Docker Server 直接运行 Server，所有路径、网络、证书和进程生命周期由容器与宿主机管理。Web 界面不知道任意宿主机目录，因此不能替代挂载配置。

两种方式共享核心 API 和媒体模型，但不具有完全相同的宿主管理面。文档在写“设置”时必须区分 Web 系统设置、Desktop 主机设置和 Server TOML。

从 Desktop 迁移到 Docker 时，不能只复制 UI 偏好；需要迁移媒体、数据库、身份和服务配置。
