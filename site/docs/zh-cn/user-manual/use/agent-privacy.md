---
title: "Lumilio Agent 的隐私边界"
description: "区分发送给模型提供方的对话与工具摘要、Server 内部引用和媒体原件。"
page_id: "use/agent-privacy"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/agent
- server/internal/agent/tools
- server/internal/service/settings_service.go
- server/internal/secretbox
-->

# Lumilio Agent 的隐私边界

Lumilio Agent 会把用户对话和模型完成任务所需的工具结果提供给配置的模型服务。使用远程 Ark、OpenAI 或 DeepSeek 时，这些数据会离开流明集 Server；使用 Ollama 时，数据仍会发往所配置的 Ollama 地址。

服务使用受授权、可过期的结果引用在前端展示媒体，避免把完整媒体数据作为额外旁路塞入对话状态。当前工具主要返回标识、筛选结果和媒体摘要；这不等于所有元数据都留在本机。

## 启用前确认

1. 阅读所选提供方的数据处理条款。
2. 确认 Base URL 指向预期服务。
3. 使用专用 API Key，并限制额度和日志保留。
4. 不在提示中输入恢复代码、密码、密钥或私密文件路径。
5. 对敏感图库优先使用受控本地提供方，或保持 Agent 关闭。

<!-- TODO(agent-privacy-contract): 当前产品缺少按工具列出的“发送给模型字段”清单和运行时预览。可选方向：请求前数据披露、字段级脱敏、本地模型模式标识、审计导出。本文采用保守表述，不承诺原件或全部元数据绝不离开设备。 -->
