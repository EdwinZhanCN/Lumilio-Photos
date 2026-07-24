// Package settings defines the runtime-mutable settings domain: the typed
// values whose single source of truth is the database `settings` table and
// which are changed at runtime through the API (Settings tabs + Setup), never
// through TOML. The immutable boot configuration lives in server/config.
//
// This package sits below both internal/service and internal/queue so that the
// MLConfigProvider interface (in queue) and the settings service (in service)
// can share these types without an import cycle. It depends on nothing internal.
package settings

import "strings"

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
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		return "ark"
	}
	return provider
}

func (c LLM) IsConfigured() bool {
	if strings.TrimSpace(c.ModelName) == "" {
		return false
	}
	switch c.EffectiveProvider() {
	case "ollama":
		return strings.TrimSpace(c.BaseURL) != ""
	default:
		return strings.TrimSpace(c.APIKey) != ""
	}
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
// the settings table's column defaults (see migration 000007), so this type has
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
	LLM LLM
	ML  ML
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
		LLM: LLM{},
		ML:  ml,
	}
}
