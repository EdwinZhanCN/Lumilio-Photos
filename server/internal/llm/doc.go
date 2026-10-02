// Package llm adapts the configured chat-model provider for the Lumilio Agent.
// [NewChatModel] builds a tool-calling Eino chat model from runtime settings
// (OpenAI-compatible, Claude, Gemini, DeepSeek, Qwen, Ark, OpenRouter, or
// Ollama) and [ValidateChatModel] probes a configuration before it is saved.
// Provider failures are normalized so callers can map them to Problems.
//
//atlas:group domain
package llm
