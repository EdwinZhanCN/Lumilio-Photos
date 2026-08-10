---
title: "浏览器无法打开流明集"
description: "区分进程未启动、监听地址错误、端口、防火墙和反向代理问题。"
page_id: "troubleshooting/cannot-open"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/config/examples/docker/http.toml
- desktop/internal/runtime/runtimeconfig
- deploy/compose/compose.yml
- deploy/compose/caddy.compose.yml
-->

# 浏览器无法打开流明集

先从 Server 所在设备测试健康检查，再从远程设备测试。这样可以区分应用问题和网络问题。

## 本机检查

- Desktop 默认仅本机模式监听 `127.0.0.1:6680`，其他设备无法访问是预期行为。
- Docker 默认配置监听 `:6680`，官方 Compose 使用主机网络。
- 检查 `/api/v1/health/live` 与 `/api/v1/health/ready` 的结果；存活不代表数据库和初始化已经就绪。

## 远程检查

确认 Server 处于局域网监听模式、宿主机防火墙允许目标端口、访问地址指向 Server 而不是浏览器设备。使用 Caddy 或内置 ACME 时，再检查域名解析、80/443 端口和证书日志。

## 不要这样做

不要先删除浏览器数据、数据库或资源库。网络不可达与媒体数据无关。反向代理返回错误时，保留代理日志和 Server 请求时间，再检查上游地址。
