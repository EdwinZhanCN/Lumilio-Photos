---
title: "Lumilio Agent 的权限与确认"
description: "理解读取工具、改变资源库内容的工具、确认流程与当前撤销边界。"
page_id: "use/agent-permissions"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/agent/core/tool_registry.go
- server/internal/agent/core/effect_runtime.go
- server/internal/agent/tools/effect_policy_test.go
- server/internal/agent/core
- web/src/features/lumilio
-->

# Lumilio Agent 的权限与确认

Agent 不会仅凭模型文字直接修改数据库。它只能调用服务注册的工具，并沿用当前登录用户的授权范围。

当前四个会改变资源库内容的 Agent 工具——批量喜欢、创建相册、添加到相册和添加标签——都在效果策略中声明需要确认并标记为可逆。执行前，界面会展示拟执行效果；只有用户确认后，服务才提交事务并写入效果回执。

## 安全使用

- 先让 Agent 展示或描述候选，再提出修改。
- 检查数量、范围和目标相册或标签。
- 不要把模型生成的解释当作 Server 已执行的事实。
- 操作完成后在资源库中验证结果。
- 当前没有经过代码验证的通用“撤销 Agent 操作”端点或界面；发生误操作时，应使用资源库现有编辑能力人工恢复，并保留效果 ID。

读取工具仍可能把资源库元数据摘要用于模型推理；权限控制不等于外部数据零披露。相关边界见[Agent 隐私](./agent-privacy.md)。

::: warning “可逆”尚不等于“已提供撤销”
效果策略中的 `reversible: true` 是设计元数据。当前回执没有保存恢复全部旧状态所需的信息，公开路由和 Web 界面也没有通用撤销操作。不要把该标记理解为用户现在可以一键回滚。
:::

<!-- TODO(agent-undo): 当前四个效果策略声明可逆，但没有公开撤销 API、完整逆操作载荷或操作历史入口。可选方向：为每类效果保存 before-state 并实现幂等 undo；或在实现前把策略改为不可逆并调整界面文案。 -->
