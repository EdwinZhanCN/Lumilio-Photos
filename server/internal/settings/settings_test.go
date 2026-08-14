package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestLLMConfigurationRequiresExplicitSupportedProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  LLM
		ok   bool
	}{
		{name: "empty provider", cfg: LLM{APIKey: "secret", ModelName: "model"}},
		{name: "unknown provider", cfg: LLM{Provider: "other", APIKey: "secret", ModelName: "model"}},
		{name: "openai", cfg: LLM{Provider: "openai", APIKey: "secret", ModelName: "model"}, ok: true},
		{name: "deepseek requires URL", cfg: LLM{Provider: "deepseek", APIKey: "secret", ModelName: "model"}},
		{name: "deepseek", cfg: LLM{Provider: "deepseek", APIKey: "secret", ModelName: "model", BaseURL: "https://deepseek.example/v1"}, ok: true},
		{name: "ollama requires URL", cfg: LLM{Provider: "ollama", ModelName: "model"}},
		{name: "ollama", cfg: LLM{Provider: " OLLAMA ", ModelName: "model", BaseURL: "http://localhost:11434"}, ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.ValidateConfiguration()
			if tt.ok && err != nil {
				t.Fatalf("ValidateConfiguration() error = %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("ValidateConfiguration() unexpectedly succeeded")
			}
			if got := tt.cfg.IsConfigured(); got != tt.ok {
				t.Fatalf("IsConfigured() = %t, want %t", got, tt.ok)
			}
		})
	}
}

func TestGeocodingDefaultsAndNormalization(t *testing.T) {
	t.Parallel()

	defaults := DefaultGeocoding()
	if defaults.Provider != GeocodingProviderDisabled ||
		defaults.NominatimEndpoint != DefaultGeocodingEndpoint ||
		defaults.Language != DefaultGeocodingLanguage ||
		defaults.UserAgent != DefaultGeocodingUserAgent {
		t.Fatalf("unexpected geocoding defaults: %+v", defaults)
	}

	normalized, err := (Geocoding{
		Provider:          " NOMINATIM ",
		NominatimEndpoint: "HTTPS://Example.COM/reverse?countrycodes=us",
		Language:          " ZH ",
		UserAgent:         " Lumilio-Test/1.0 ",
	}).Normalize()
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if normalized.Provider != GeocodingProviderNominatim ||
		normalized.NominatimEndpoint != "https://example.com/reverse?countrycodes=us" ||
		normalized.Language != "zh" || normalized.UserAgent != "Lumilio-Test/1.0" {
		t.Fatalf("unexpected normalized geocoding: %+v", normalized)
	}

	wantDigest := sha256.Sum256([]byte(normalized.Provider + normalized.NominatimEndpoint + normalized.Language))
	if normalized.SourceKey() != hex.EncodeToString(wantDigest[:]) {
		t.Fatalf("SourceKey() = %q, want SHA-256 of the normalized source", normalized.SourceKey())
	}
	otherUserAgent := normalized
	otherUserAgent.UserAgent = "another-agent"
	if otherUserAgent.SourceKey() != normalized.SourceKey() {
		t.Fatal("User-Agent changed the geocoding source key")
	}
}

func TestGeocodingValidation(t *testing.T) {
	t.Parallel()

	base := DefaultGeocoding()
	tests := []struct {
		name   string
		mutate func(*Geocoding)
	}{
		{name: "provider", mutate: func(cfg *Geocoding) { cfg.Provider = "google" }},
		{name: "relative endpoint", mutate: func(cfg *Geocoding) { cfg.NominatimEndpoint = "/reverse" }},
		{name: "unsupported scheme", mutate: func(cfg *Geocoding) { cfg.NominatimEndpoint = "ftp://example.test/reverse" }},
		{name: "missing host", mutate: func(cfg *Geocoding) { cfg.NominatimEndpoint = "https:///reverse" }},
		{name: "credentials", mutate: func(cfg *Geocoding) { cfg.NominatimEndpoint = "https://user:pass@example.test/reverse" }},
		{name: "fragment", mutate: func(cfg *Geocoding) { cfg.NominatimEndpoint = "https://example.test/reverse#fragment" }},
		{name: "language controls", mutate: func(cfg *Geocoding) { cfg.Language = "en\nUS" }},
		{name: "language leading controls", mutate: func(cfg *Geocoding) { cfg.Language = "\ten" }},
		{name: "user agent controls", mutate: func(cfg *Geocoding) { cfg.UserAgent = "Lumilio\tPhotos" }},
		{name: "user agent trailing controls", mutate: func(cfg *Geocoding) { cfg.UserAgent = "Lumilio-Photos/1.0\r" }},
		{name: "user agent too long", mutate: func(cfg *Geocoding) { cfg.UserAgent = strings.Repeat("x", MaxGeocodingUserAgentBytes+1) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() unexpectedly succeeded")
			}
		})
	}

	loopback, err := (Geocoding{
		Provider:          GeocodingProviderNominatim,
		NominatimEndpoint: "http://127.0.0.1:8080/reverse",
		Language:          "en",
		UserAgent:         DefaultGeocodingUserAgent,
	}).Normalize()
	if err != nil {
		t.Fatalf("loopback endpoint rejected: %v", err)
	}
	if loopback.NominatimEndpoint != "http://127.0.0.1:8080/reverse" {
		t.Fatalf("loopback endpoint normalized to %q", loopback.NominatimEndpoint)
	}
}
