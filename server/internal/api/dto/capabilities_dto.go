package dto

import "time"

// CapabilitiesResponseDTO represents the public, de-sensitized runtime capability view.
type CapabilitiesResponseDTO struct {
	ML  MLCapabilitiesDTO  `json:"ml"`
	LLM LLMCapabilitiesDTO `json:"llm"`
}

// MLCapabilitiesDTO represents ML runtime task availability and discovery state.
type MLCapabilitiesDTO struct {
	DiscoveryState        string       `json:"discovery_state" enums:"disabled,starting,healthy,degraded" example:"healthy"`
	DiscoveredNodeCount   int          `json:"discovered_node_count" example:"2"`
	ActiveNodeCount       int          `json:"active_node_count" example:"1"`
	ConnectingNodeCount   int          `json:"connecting_node_count" example:"0"`
	UnavailableNodeCount  int          `json:"unavailable_node_count" example:"0"`
	PendingNodeCount      int          `json:"pending_node_count" example:"1"`
	IncompatibleNodeCount int          `json:"incompatible_node_count" example:"0"`
	Tasks                 MLTaskSetDTO `json:"tasks"`
}

// MLTaskSetDTO groups the known ML task capabilities that Lumilio can use.
type MLTaskSetDTO struct {
	SemanticImageEmbed MLTaskCapabilityDTO `json:"semantic_image_embed"`
	SemanticTextEmbed  MLTaskCapabilityDTO `json:"semantic_text_embed"`
	BioClipClassify    MLTaskCapabilityDTO `json:"bioclip_classify"`
	OCR                MLTaskCapabilityDTO `json:"ocr"`
	FaceRecognition    MLTaskCapabilityDTO `json:"face_recognition"`
}

// MLTaskCapabilityDTO represents enablement and real-time availability for a single ML task.
type MLTaskCapabilityDTO struct {
	Enabled   bool `json:"enabled"`
	Available bool `json:"available"`
}

// LLMCapabilitiesDTO represents de-sensitized LLM agent runtime state.
type LLMCapabilitiesDTO struct {
	Availability string `json:"availability" enums:"disabled,not_configured,ready" example:"ready"`
	AgentEnabled bool   `json:"agent_enabled"`
	Configured   bool   `json:"configured"`
	Provider     string `json:"provider,omitempty" example:"openai"`
	ModelName    string `json:"model_name,omitempty" example:"gpt-4.1-mini"`
}

// LumenRuntimeDTO is the authenticated administrator projection of the SDK
// runtime snapshot. It intentionally excludes TXT metadata and raw errors.
type LumenRuntimeDTO struct {
	CapturedAt     time.Time               `json:"captured_at"`
	DiscoveryState string                  `json:"discovery_state" enums:"disabled,starting,healthy,degraded"`
	Counts         LumenRuntimeCountsDTO   `json:"counts"`
	Backends       []LumenBackendStatusDTO `json:"backends"`
	Nodes          []LumenNodeRuntimeDTO   `json:"nodes"`
}

type LumenRuntimeCountsDTO struct {
	Discovered   int `json:"discovered"`
	Active       int `json:"active"`
	Connecting   int `json:"connecting"`
	Unavailable  int `json:"unavailable"`
	Pending      int `json:"pending"`
	Incompatible int `json:"incompatible"`
}

type LumenBackendStatusDTO struct {
	Source              string     `json:"source"`
	State               string     `json:"state" enums:"disabled,starting,healthy,degraded"`
	LastScanStartedAt   *time.Time `json:"last_scan_started_at,omitempty"`
	LastScanCompletedAt *time.Time `json:"last_scan_completed_at,omitempty"`
	LastScanSucceededAt *time.Time `json:"last_scan_succeeded_at,omitempty"`
	NextScanAt          *time.Time `json:"next_scan_at,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	LastErrorCode       string     `json:"last_error_code,omitempty"`
	MatchedCount        int        `json:"matched_count"`
	RejectedCount       int        `json:"rejected_count"`
	LastOutcome         string     `json:"last_outcome,omitempty" enums:"success,failed,timed_out,cancelled"`
}

type LumenNodeRuntimeDTO struct {
	ID             string             `json:"id"`
	Endpoint       string             `json:"endpoint"`
	Sources        []string           `json:"sources"`
	LastObservedAt *time.Time         `json:"last_observed_at,omitempty"`
	UpdatedAt      *time.Time         `json:"updated_at,omitempty"`
	Transport      string             `json:"transport" enums:"connecting,ready,unavailable"`
	Compatibility  string             `json:"compatibility" enums:"pending,compatible,incompatible"`
	Version        string             `json:"version,omitempty"`
	Runtime        string             `json:"runtime,omitempty"`
	ErrorCode      string             `json:"error_code,omitempty"`
	Tasks          []LumenNodeTaskDTO `json:"tasks"`
}

type LumenNodeTaskDTO struct {
	Service string `json:"service"`
	Task    string `json:"task"`
}
