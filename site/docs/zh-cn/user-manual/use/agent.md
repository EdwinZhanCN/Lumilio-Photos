---
title: "Lumilio Agent"
description: "通过受控工具检索、审阅和整理媒体，并理解对话与持久状态边界。"
page_id: "use/agent"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/agent
- server/internal/agent/core/effect_runtime.go
- server/internal/api/router.go
- web/src/features/lumilio
- server/internal/api/handler/capabilities_handler.go
-->

# Lumilio Agent

Lumilio Agent 使用管理员配置的模型提供方理解请求，再调用流明集内置工具检索或整理媒体。新对话只有在 Agent 开关、提供方、模型和网络验证均就绪时可用。

## 可以做什么

Agent 工具包括元数据和文本筛选、人物或相册查询、语义搜索、抽样、排序、去重、描述和展示结果。当前可改变图库的工具包括批量喜欢、创建相册、添加到相册和添加标签。

## 对话与持久状态

对话线程和运行状态会写入数据库；固定到看板的小组件是独立的持久对象。对话中的结果引用受用户和线程授权约束，并可能过期，因此不要把聊天内容当作永久媒体清单。已提交效果的回执会在服务端保留一段有限时间用于断线对账，当前实现会清理超过 30 天的已结束回执。

## 能力退化

语义工具依赖 Lumen Intelligence 的嵌入能力；普通元数据查询和部分组织工具不一定依赖同一能力。Agent 页面不可用时，应先区分模型提供方问题、工具能力问题和 Lumen 任务问题。
