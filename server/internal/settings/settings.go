// Package settings defines the runtime-mutable settings domain: the typed
// values whose single source of truth is the database `settings` table and
// which are changed at runtime through the API (Settings tabs + Setup), never
// through TOML. The immutable boot configuration lives in server/config.
//
// This package sits below both internal/service and internal/queue so that the
// MLConfigProvider interface (in queue) and the settings service (in service)
// can share these types without an import cycle. It depends on nothing internal.
package settings

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	GeocodingProviderDisabled  = "disabled"
	GeocodingProviderNominatim = "nominatim"
	DefaultGeocodingEndpoint   = "https://nominatim.openstreetmap.org/reverse"
	DefaultGeocodingLanguage   = "en"
	DefaultGeocodingUserAgent  = "Lumilio-Photos/1.0"
	MaxGeocodingEndpointBytes  = 2048
	MaxGeocodingLanguageBytes  = 64
	MaxGeocodingUserAgentBytes = 512
)

// Geocoding is the complete administrator-owned reverse-geocoding aggregate.
// It is persisted in SQLite and deliberately has no representation in the
// runtime-immutable TOML manifest.
type Geocoding struct {
	Provider          string
	NominatimEndpoint string
	Language          string
	UserAgent         string
}

func DefaultGeocoding() Geocoding {
	return Geocoding{
		Provider:          GeocodingProviderDisabled,
		NominatimEndpoint: DefaultGeocodingEndpoint,
		Language:          DefaultGeocodingLanguage,
		UserAgent:         DefaultGeocodingUserAgent,
	}
}

// Normalize validates and canonicalizes one aggregate before persistence or
// comparison. The endpoint intentionally permits loopback and private-network
// hosts: an authenticated administrator may operate a local Nominatim server.
func (c Geocoding) Normalize() (Geocoding, error) {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider != GeocodingProviderDisabled && provider != GeocodingProviderNominatim {
		return Geocoding{}, fmt.Errorf("geocoding provider must be %q or %q", GeocodingProviderDisabled, GeocodingProviderNominatim)
	}

	endpoint, err := normalizeGeocodingEndpoint(c.NominatimEndpoint)
	if err != nil {
		return Geocoding{}, err
	}
	if err := validateGeocodingText("language", c.Language, MaxGeocodingLanguageBytes, true); err != nil {
		return Geocoding{}, err
	}
	if err := validateGeocodingText("user agent", c.UserAgent, MaxGeocodingUserAgentBytes, true); err != nil {
		return Geocoding{}, err
	}
	language := strings.ToLower(strings.TrimSpace(c.Language))
	userAgent := strings.TrimSpace(c.UserAgent)

	return Geocoding{
		Provider:          provider,
		NominatimEndpoint: endpoint,
		Language:          language,
		UserAgent:         userAgent,
	}, nil
}

func (c Geocoding) Validate() error {
	_, err := c.Normalize()
	return err
}

func (c Geocoding) IsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(c.Provider), GeocodingProviderNominatim)
}

// SourceKey identifies the provider result source. User-Agent is excluded by
// design: it changes request identity, not the place-name result identity.
func (c Geocoding) SourceKey() string {
	normalized, err := c.Normalize()
	if err != nil {
		return ""
	}
	digest := sha256.Sum256([]byte(normalized.Provider + normalized.NominatimEndpoint + normalized.Language))
	return hex.EncodeToString(digest[:])
}

func normalizeGeocodingEndpoint(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if err := validateGeocodingText("endpoint", trimmed, MaxGeocodingEndpointBytes, false); err != nil {
		return "", err
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("geocoding endpoint is not a valid URL: %w", err)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("geocoding endpoint must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("geocoding endpoint must have a host")
	}
	if parsed.User != nil {
		return "", errors.New("geocoding endpoint must not contain credentials")
	}
	if parsed.Fragment != "" {
		return "", errors.New("geocoding endpoint must not contain a fragment")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	canonical := parsed.String()
	if len([]byte(canonical)) > MaxGeocodingEndpointBytes {
		return "", fmt.Errorf("geocoding endpoint must be at most %d bytes", MaxGeocodingEndpointBytes)
	}
	return canonical, nil
}

func validateGeocodingText(name, value string, maxBytes int, rejectHeaderControls bool) error {
	if rejectHeaderControls {
		for _, byteValue := range []byte(value) {
			if byteValue < 0x20 || byteValue == 0x7f {
				return fmt.Errorf("geocoding %s contains invalid HTTP header bytes", name)
			}
		}
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("geocoding %s must be non-empty", name)
	}
	if len([]byte(value)) > maxBytes {
		return fmt.Errorf("geocoding %s must be at most %d bytes", name, maxBytes)
	}
	return nil
}

// LLM holds the effective LLM settings, including the plaintext API key needed
// to construct a chat model. The API surface never exposes the key directly;
// callers that only report configured-state use IsConfigured.
type LLM struct {
	AgentEnabled bool
	Provider     string
	APIKey       string
	ModelName    string
	BaseURL      string
}

func (c LLM) EffectiveProvider() string {
	return NormalizeLLMProvider(c.Provider)
}

func (c LLM) ValidateConfiguration() error {
	provider := c.EffectiveProvider()
	descriptor, ok := LookupLLMProvider(provider)
	if !ok {
		if provider == "" {
			return fmt.Errorf("llm provider is required")
		}
		return fmt.Errorf("unsupported llm provider %q", provider)
	}
	if strings.TrimSpace(c.ModelName) == "" {
		return fmt.Errorf("llm model name is required")
	}
	if descriptor.BaseURLRequired && strings.TrimSpace(c.BaseURL) == "" {
		return fmt.Errorf("%s base URL is required", provider)
	}
	if descriptor.APIKeyRequired && strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("llm API key is required for provider %q", provider)
	}
	return nil
}

func (c LLM) IsConfigured() bool {
	return c.ValidateConfiguration() == nil
}

// ML holds the runtime ML task toggles and video-semantic sampling knobs.
// Zero-shot classification has no separate toggle: it is gated by
// SemanticEnabled (the classify job is enqueued only after a successful
// semantic embed). Video frame embedding requires both SemanticEnabled and
// VideoSemanticEnabled.
type ML struct {
	SemanticEnabled           bool
	BioCLIPEnabled            bool
	OCREnabled                bool
	FaceEnabled               bool
	VideoSemanticEnabled      bool
	VideoMaxFrames            int
	VideoLongThresholdSeconds int
	VideoSceneThreshold       float64
}

const (
	DefaultVideoMaxFrames            = 8
	DefaultVideoLongThresholdSeconds = 300
	DefaultVideoSceneThreshold       = 0.4
)

func (c ML) HasManualTasksEnabled() bool {
	return c.SemanticEnabled || c.BioCLIPEnabled || c.OCREnabled || c.FaceEnabled ||
		(c.SemanticEnabled && c.VideoSemanticEnabled)
}

// EffectiveVideoMaxFrames returns a positive frame cap, falling back to the
// default when the stored value is unset or invalid.
func (c ML) EffectiveVideoMaxFrames() int {
	if c.VideoMaxFrames <= 0 {
		return DefaultVideoMaxFrames
	}
	return c.VideoMaxFrames
}

// EffectiveVideoLongThresholdSeconds returns the short/long sampling boundary.
func (c ML) EffectiveVideoLongThresholdSeconds() int {
	if c.VideoLongThresholdSeconds <= 0 {
		return DefaultVideoLongThresholdSeconds
	}
	return c.VideoLongThresholdSeconds
}

// EffectiveVideoSceneThreshold returns the ffmpeg scene-change threshold.
func (c ML) EffectiveVideoSceneThreshold() float64 {
	if c.VideoSceneThreshold <= 0 || c.VideoSceneThreshold >= 1 {
		return DefaultVideoSceneThreshold
	}
	return c.VideoSceneThreshold
}

func (c ML) HasRuntimeDemand() bool {
	return c.HasManualTasksEnabled()
}

// Backup holds the runtime-mutable database-backup settings. Seed values are
// the settings table's column defaults (see migration 000008), so this type has
// no entry in Default.
type Backup struct {
	Enabled       bool
	IntervalHours int
	KeepLast      int
}

// Settings is the full set of runtime-mutable settings owned by the settings
// service. Repository behaviour defaults are owned by the storage package, not
// here.
type Settings struct {
	LLM       LLM
	ML        ML
	Geocoding Geocoding
}

// Default returns the program-fixed default settings used to seed the database
// on first run. ML defaults differ by environment: production enables ML tasks,
// development disables them so local dev does not require an ML node.
func Default(environment string) Settings {
	ml := ML{
		SemanticEnabled:           true,
		BioCLIPEnabled:            true,
		OCREnabled:                true,
		FaceEnabled:               true,
		VideoSemanticEnabled:      true,
		VideoMaxFrames:            DefaultVideoMaxFrames,
		VideoLongThresholdSeconds: DefaultVideoLongThresholdSeconds,
		VideoSceneThreshold:       DefaultVideoSceneThreshold,
	}
	if strings.EqualFold(strings.TrimSpace(environment), "development") {
		// Zero toggles so local dev does not require an ML node; keep sampling
		// knobs at their defaults so a later enable works without re-seed.
		ml = ML{
			VideoMaxFrames:            DefaultVideoMaxFrames,
			VideoLongThresholdSeconds: DefaultVideoLongThresholdSeconds,
			VideoSceneThreshold:       DefaultVideoSceneThreshold,
		}
	}
	return Settings{
		LLM:       LLM{},
		ML:        ml,
		Geocoding: DefaultGeocoding(),
	}
}
