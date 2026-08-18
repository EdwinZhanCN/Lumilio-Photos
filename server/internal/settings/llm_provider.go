package settings

import "strings"

const (
	LLMProviderArk        = "ark"
	LLMProviderOpenAI     = "openai"
	LLMProviderDeepSeek   = "deepseek"
	LLMProviderOllama     = "ollama"
	LLMProviderClaude     = "claude"
	LLMProviderGemini     = "gemini"
	LLMProviderQwen       = "qwen"
	LLMProviderOpenRouter = "openrouter"
)

// LLMProviderDescriptor is the Server-owned contract for one supported chat
// model provider. The API publishes these requirements so consumers do not
// duplicate provider-specific validation rules.
type LLMProviderDescriptor struct {
	ID              string
	APIKeyRequired  bool
	BaseURLRequired bool
}

var supportedLLMProviders = [...]LLMProviderDescriptor{
	{ID: LLMProviderArk, APIKeyRequired: true},
	{ID: LLMProviderOpenAI, APIKeyRequired: true},
	{ID: LLMProviderDeepSeek, APIKeyRequired: true, BaseURLRequired: true},
	{ID: LLMProviderOllama, BaseURLRequired: true},
	{ID: LLMProviderClaude, APIKeyRequired: true},
	{ID: LLMProviderGemini, APIKeyRequired: true},
	{ID: LLMProviderQwen, APIKeyRequired: true, BaseURLRequired: true},
	{ID: LLMProviderOpenRouter, APIKeyRequired: true},
}

func NormalizeLLMProvider(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// SupportedLLMProviders returns a copy so the registry remains immutable to
// callers while preserving its deterministic product order.
func SupportedLLMProviders() []LLMProviderDescriptor {
	providers := make([]LLMProviderDescriptor, len(supportedLLMProviders))
	copy(providers, supportedLLMProviders[:])
	return providers
}

func LookupLLMProvider(raw string) (LLMProviderDescriptor, bool) {
	provider := NormalizeLLMProvider(raw)
	for _, descriptor := range supportedLLMProviders {
		if descriptor.ID == provider {
			return descriptor, true
		}
	}
	return LLMProviderDescriptor{}, false
}

func IsSupportedLLMProvider(provider string) bool {
	_, ok := LookupLLMProvider(provider)
	return ok
}
