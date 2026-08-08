---
title: "安全边界"
description: "概览账户、令牌、公开分享、云凭据、模型提供方和文件系统的信任边界。"
page_id: "reference/security-boundaries"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/auth_service.go
- server/internal/service/share_link_service.go
- server/internal/api/handler/share_link_handler.go
- server/internal/cloud
- server/internal/agent
- server/internal/service/lumen_service.go
-->

# 安全边界

## 认证会话

访问令牌短期有效，刷新令牌轮换，媒体令牌单独短期授权。密码变化、访问重置和令牌重用会撤销相应会话。

## 公开入口

公开注册和分享链接不要求已有登录。分享由高熵令牌控制；注册当前没有关闭开关。公开分享的原件策略目前执行不一致：ZIP 下载只检查“允许下载”，视频/音频派生缺失时会回退原件。部署者必须控制服务可达范围，并把分享令牌视作能够读取快照媒体的凭据。

## 文件系统

媒体存储对 Server 可读写；应用私有状态包含数据库和秘密，应使用更严格权限。Docker 以 UID/GID `10001` 运行。

## 外部服务

云凭据用于连接 iCloud，会话保存在隔离目录。Lumilio Agent 会与配置的模型提供方通信；Lumen Intelligence 节点通过 gRPC 接收任务数据。网络位置必须按实际部署评估。

## HTTPS

HTTP 会暴露密码、TOTP、会话和媒体流量。远程访问和通行密钥应使用 HTTPS 域名。
