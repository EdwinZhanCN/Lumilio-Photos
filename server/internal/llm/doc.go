// Package llm adapts the configured chat-model provider for the Lumilio Agent.
// [NewChatModel] builds a tool-calling Eino chat model from runtime settings
// (OpenAI-compatible, Claude, Gemini, DeepSeek, Qwen, Ark, OpenRouter, or
// Ollama) and [ValidateChatModel] probes a configuration before it is saved.
// Provider failures are normalized so callers can map them to Problems.
//
// Provider SDK failures are classified here before they reach HTTP or Agent
// logging: ordinary logs never contain provider response bodies, Authorization
// values, or prompts reflected by a remote service. Each adapter is the
// official Eino-ext one; Claude uses only the direct Anthropic path, Gemini an
// explicitly constructed Developer API client, and DeepSeek its native
// adapter. Hosted variants whose credentials need ambient cloud state stay
// unsupported.
//
//atlas:group domain
package llm
