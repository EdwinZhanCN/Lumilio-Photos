---
title: "收集诊断信息"
description: "形成可复现、可脱敏且足以定位问题的最小证据集。"
page_id: "troubleshooting/collect-diagnostics"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/logging/logger.go
- server/internal/api/handler/queue_handler.go
- server/internal/api/handler/health_handler.go
- server/internal/api/handler/capabilities_handler.go
-->

# 收集诊断信息

## 基本信息

- baseline、版本或镜像标签；
- Desktop / Docker、操作系统与 CPU 架构；
- 问题发生的绝对时间和时区；
- 页面与操作步骤；
- 是否每次复现；
- 资源库 ID、存储位置状态和相关任务名；
- 预期结果与实际结果。

## 日志与状态

优先提供问题时间前后的小段日志、健康与能力状态、队列错误样本、恢复操作 ID 或扫描结果。不要上传整个数据库、原件、云会话目录或密钥目录。

## 脱敏

删除密码、临时密码、恢复代码、API Key、Cookie、CSRF 值、刷新令牌、分享令牌、完整私人路径和照片内容。保留错误代码、组件名、时间、匿名 ID 和必要路径结构。

## 最小复现

对媒体问题，使用可公开的小文件副本；对存储问题，画出宿主机路径、容器挂载和资源库目录；对网络问题，分别记录 Server 本机和远程客户端结果。
