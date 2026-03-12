package dto

// CapabilitiesResponseDTO represents the public, de-sensitized runtime capability view.
type CapabilitiesResponseDTO struct {
	ML  MLCapabilitiesDTO  `json:"ml"`
	LLM LLMCapabilitiesDTO `json:"llm"`
}

// MLCapabilitiesDTO represents ML runtime task availability and discovery state.
type MLCapabilitiesDTO struct {
	AutoMode            string       `json:"auto_mode" example:"enable"`
	DiscoveredNodeCount int          `json:"discovered_node_count" example:"2"`
	ActiveNodeCount     int          `json:"active_node_count" example:"1"`
	Tasks               MLTaskSetDTO `json:"tasks"`
}

// MLTaskSetDTO groups the known ML task capabilities that Lumilio can use.
type MLTaskSetDTO struct {
	ClipImageEmbed     MLTaskCapabilityDTO `json:"clip_image_embed"`
	ClipTextEmbed      MLTaskCapabilityDTO `json:"clip_text_embed"`
	ClipClassify       MLTaskCapabilityDTO `json:"clip_classify"`
	ClipSceneClassify  MLTaskCapabilityDTO `json:"clip_scene_classify"`
	OCR                MLTaskCapabilityDTO `json:"ocr"`
	VLMGenerate        MLTaskCapabilityDTO `json:"vlm_generate"`
	FaceDetectAndEmbed MLTaskCapabilityDTO `json:"face_detect_and_embed"`
}

// MLTaskCapabilityDTO represents enablement and real-time availability for a single ML task.
type MLTaskCapabilityDTO struct {
	Enabled   bool `json:"enabled"`
	Available bool `json:"available"`
}

// LLMCapabilitiesDTO represents de-sensitized LLM agent runtime state.
type LLMCapabilitiesDTO struct {
	AgentEnabled bool   `json:"agent_enabled"`
	Configured   bool   `json:"configured"`
	Provider     string `json:"provider,omitempty" example:"openai"`
	ModelName    string `json:"model_name,omitempty" example:"gpt-4.1-mini"`
}
