---
title: "安装 Docker Server"
description: "使用官方 Compose 建立可持久化、可检查的流明集 Server。"
page_id: "getting-started/install-docker"
audience: "管理员"
platform: "Linux Docker"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- deploy/compose/compose.yml
- server/Dockerfile
- server/docker-entrypoint.go
- server/config/examples/docker/http.toml
- server/internal/api/router.go
-->

# 安装 Docker Server

默认 Docker 入口适合先在可信局域网中验证。它使用内置完整配置，通常不需要先编写 TOML；你必须明确保存媒体和应用私有状态。

## 准备目录

至少准备两个独立宿主机路径：

- 媒体存储，挂载到容器内 `/data/storage`；
- 应用私有状态，挂载到 `/data/app-state`。

官方 Compose 对应的环境变量是 `LUMILIO_STORAGE` 和 `LUMILIO_STATE`。未设置时会使用项目目录下的 `./lumilio/media` 与 `./lumilio/app-state`。

## 启动

1. 复制仓库中的 `deploy/compose/compose.yml`，或从同一 baseline 获取它。
2. 设置两个宿主机路径，并确认 Docker 可以写入。
3. 启动 Compose。
4. 观察健康检查通过后，访问宿主机的 `6680` 端口。
5. 完成管理员和主资源库设置。

容器以 UID/GID `10001` 运行。入口会检查两个挂载是否可写，并尝试修正挂载顶层权限；如果宿主文件系统拒绝，它会停止并输出可操作的权限错误。不要通过让整个媒体树永久变为任意用户可写来规避权限问题。

::: warning 默认网络与注册边界
默认 Compose 使用主机网络并提供 HTTP。当前代码在初始化后仍显示公开注册入口；因此不要把未加访问控制的默认实例直接暴露到不可信网络。完成首次验证后，应阅读[HTTPS 与访问边界](../admin/https.md)和[公开注册风险](../admin/registration-exposure.md)。
:::

<!-- TODO(release-reproducibility): 默认 Compose 镜像标签指向 latest。可选方向：文档要求固定版本标签，或由发布包生成带版本锁定的 Compose。当前页建议使用与目标 baseline 对应的发布标签，但代码仓库没有提供唯一自动替换值。 -->
