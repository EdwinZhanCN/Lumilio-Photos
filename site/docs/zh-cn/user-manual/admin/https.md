---
title: "HTTPS 与网络访问"
description: "在仅本机、可信局域网、Caddy 或内置 ACME 之间选择，并支持通行密钥。"
page_id: "admin/https"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/httporigin/policy.go
- server/config
- deploy/compose/caddy.compose.yml
- deploy/compose/acme.compose.yml
- desktop/internal/runtime/runtimeconfig
-->

# HTTPS 与网络访问

HTTP 会在网络中暴露密码、TOTP、会话和媒体请求。仅在 `localhost` 或完全可信、受控的临时局域网中使用默认 HTTP；长期远程访问应使用 HTTPS。

## 仅本机

Desktop 默认监听 `127.0.0.1:6680`，只允许同一电脑访问。`localhost` 也是通行密钥安全来源。

## 可信局域网

Desktop 可以切换局域网监听，Docker 默认监听 `:6680`。这不提供加密；浏览器会显示相应风险。不要把“只在家里”自动等同于可信。

## Caddy 方案

项目的 Caddy 叠加配置把流明集上游限制在 `127.0.0.1:6680`，由 Caddy 占用 80/443，并要求设置 `LUMILIO_DOMAIN`。确认 DNS 指向主机，外部只开放 Caddy 端口。

## 内置 ACME

内置 ACME 配置需要生成完整 TOML，并让 Server 直接绑定 80/443。证书获取失败会使启动失败，因此上线前要验证 DNS、端口和持久证书状态。

## 通行密钥

远程通行密钥必须使用 HTTPS 域名；裸 IP 即使有证书也被当前来源校验拒绝。反向代理必须把外部来源信息正确传给应用。
