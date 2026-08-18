package dto

import (
	"reflect"
	"testing"

	"server/internal/service"
)

func TestToSystemSettingsDTOPublishesSupportedLLMProviders(t *testing.T) {
	t.Parallel()

	got := ToSystemSettingsDTO(service.SystemSettings{}).LLM.SupportedProviders
	want := []LLMProviderDescriptorDTO{
		{ID: "ark", APIKeyRequired: true},
		{ID: "openai", APIKeyRequired: true},
		{ID: "deepseek", APIKeyRequired: true, BaseURLRequired: true},
		{ID: "ollama", BaseURLRequired: true},
		{ID: "claude", APIKeyRequired: true},
		{ID: "gemini", APIKeyRequired: true},
		{ID: "qwen", APIKeyRequired: true, BaseURLRequired: true},
		{ID: "openrouter", APIKeyRequired: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("supported providers = %#v, want %#v", got, want)
	}
}
