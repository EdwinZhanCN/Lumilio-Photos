---
title: "Lumilio Agent 不可用"
description: "区分未启用、未配置、提供方不可达和依赖工具退化。"
page_id: "troubleshooting/agent-unavailable"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/capabilities_handler.go
- server/internal/service/settings_service.go
- server/internal/agent
- server/internal/agent/core/effect_runtime.go
- web/src/features/lumilio
-->

# Lumilio Agent 不可用

Agent 能力状态至少有关闭、未配置和就绪等区别。页面打不开新对话时，先由管理员检查系统设置，而不是重建媒体索引。

## 检查顺序

1. Agent 开关已启用。
2. 提供方、模型和必要的 Base URL/API Key 已填写。
3. “验证连接”成功。
4. 流明集 Server 能访问提供方地址。
5. 当前用户会话有效。
6. 请求所需的语义或其他工具能力可用。

已有对话结果引用可能过期；重新打开旧结果失败不一定代表提供方故障。固定到看板的小组件可以从持久状态重新加载；已提交效果回执主要用于按效果 ID 对账，并会在有限保留期后清理，不能当作永久操作历史。

提供方返回限流、认证失败或模型不存在时，保留状态码和请求时间，但不要把 API Key 写入诊断材料。
