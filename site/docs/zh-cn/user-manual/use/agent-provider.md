---
title: "配置 Lumilio Agent 提供方"
description: "使用管理员设置配置 Ark、OpenAI、DeepSeek 或 Ollama，并在保存前验证。"
page_id: "use/agent-provider"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/dto/settings_dto.go
- server/internal/service/settings_service.go
- server/internal/llm/chat_model.go
- web/src/features/settings
-->

# 配置 Lumilio Agent 提供方

当前支持的提供方标识是 `ark`、`openai`、`deepseek` 和 `ollama`。所有提供方都需要模型名称；Ark 和 OpenAI 需要 API Key，DeepSeek 需要 API Key 与 Base URL，Ollama 需要 Base URL 而不要求 API Key。

## 配置步骤

1. 以管理员进入“设置 → AI”。
2. 选择提供方并填写模型。
3. 按提供方要求填写 Base URL 和 API Key。
4. 使用“验证连接”；验证请求超时为 15 秒。
5. 验证成功后保存并启用 Agent。

API Key 加密保存，读取设置时只返回“已配置”状态而不返回明文。更换提供方时，如果没有同时提交替代密钥，服务会清除旧提供方的已存密钥。

## 自托管提供方

Ollama 的可达性取决于流明集 Server 所在网络，而不是浏览器所在网络。Docker 中的 `localhost` 指容器自身；应根据网络模式填写 Server 实际可达的地址。
