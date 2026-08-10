---
title: "Lumilio Agent 权限模型"
description: "解释模型、工具、用户授权、确认和持久操作记录如何分层。"
page_id: "concepts/agent-permission-model"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/agent/core/tool_registry.go
- server/internal/agent/core/effect_runtime.go
- server/internal/agent/tools/effect_policy_test.go
- server/internal/agent/core
- server/internal/agent/ref
-->

# Lumilio Agent 权限模型

模型负责提出下一步工具调用，流明集 Server 决定哪些工具存在、当前用户能访问哪些资源，以及效果操作是否必须确认。

读取工具返回受当前用户和线程授权约束的结果。结果引用可以过期，前端需要重新授权和取回媒体；模型本身不因此获得任意资产 URL。

改变资源库内容的工具具有显式效果策略。当前四类修改工具都需要确认，并在策略元数据中声明 `reversible: true`。确认是 Server 控制面的一部分，不应由模型用自然语言替代。

已提交效果的回执会持久保存，用于流式连接中断后的状态对账；已结束回执当前会在 30 天后清理。代码没有提供通用撤销 API，也没有在回执中保存全部旧状态，所以“可逆”目前只是尚未闭环的契约声明，不能作为恢复承诺。权限模型仍不能解决外部模型提供方的数据处理问题，那属于独立的隐私边界。
