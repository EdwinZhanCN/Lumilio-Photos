// Package settings defines the runtime-mutable settings domain: typed values
// whose single source of truth is the catalog settings row, changed through
// the Settings and Setup APIs, never through TOML. The immutable boot
// configuration lives in server/config.
//
// It owns the deterministic LLM provider registry ([LookupLLMProvider],
// [LLMProviderDescriptor]): which providers exist and whether each requires an
// API key or a base URL. Ollama is the only keyless provider; Ollama and Qwen
// require explicit endpoints. Settings responses publish only those facts, and
// request DTOs defer membership checks to the registry.
// [LLM.ValidateConfiguration] rejects an incomplete setting so it can never
// fall through to SDK environment variables. Reverse geocoding ([Geocoding])
// is likewise a runtime setting.
//
// It depends on nothing internal, so the queue's ML configuration provider and
// the settings service share these types without an import cycle.
//
//atlas:group foundation
package settings
