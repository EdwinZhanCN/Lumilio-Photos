---
title: "Server 配置参考"
description: "概览严格 manifest v3 的主要配置组和修改边界。"
page_id: "reference/server-configuration"
audience: "管理员、开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/config/config.go
- server/config/schema/lumilio-server.schema.json
- server/internal/api/handler/settings_handler.go
-->

# Server 配置参考

独立 Server 使用完整、严格的 TOML manifest。配置加载不会依赖隐式环境变量覆盖来补齐缺失字段；未识别或缺失的关键结构应视为启动错误。

主要配置组包括：

- HTTP/HTTPS 监听与 ACME；
- SQLite 数据库和应用私有路径；
- 默认存储位置和资源库默认策略；
- JWT 密钥、令牌有效期、通行密钥与认证限流；
- 扫描间隔、稳定时间、批量和并发；
- 反向地理编码；
- FFmpeg 硬件加速；
- Lumen 发现、静态节点和超时；
- ExifTool、FFmpeg 与 FFprobe 路径；
- 日志和云状态路径。

设置页中的“运行时配置”是只读观察面；需要修改的值应在 Desktop 主机设置或 TOML 中调整并重启。系统数据库设置只覆盖 LLM、智能任务和数据库快照等明确可变项。

<!-- TODO(config-reference): 完整 schema 由 Go 结构和生成器定义，手写逐字段文档容易漂移。可选方向：发布时从 schema 生成带默认值、范围和重启要求的参考页。 -->
