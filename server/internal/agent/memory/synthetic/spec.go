package synthetic

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"server/internal/agent/core"
	"server/internal/agent/memory"

	"github.com/google/uuid"
)

type SpecBundle struct {
	SchemaVersion string        `json:"schema_version"`
	Episodes      []EpisodeSpec `json:"episodes"`
	Queries       []QuerySpec   `json:"queries,omitempty"`
}

const SpecSchemaVersion = "agent-memory/episode-spec-bundle/v1"

type EpisodeSpec struct {
	EpisodeID    string                `json:"episode_id"`
	ClusterID    string                `json:"cluster_id"`
	Scenario     string                `json:"scenario"`
	Goal         string                `json:"goal"`
	Intent       string                `json:"intent"`
	Summary      string                `json:"summary,omitempty"`
	Status       memory.EpisodeStatus  `json:"status,omitempty"`
	WriteTrigger memory.WriteTrigger   `json:"write_trigger,omitempty"`
	AgentName    string                `json:"agent_name,omitempty"`
	Workspace    string                `json:"workspace,omitempty"`
	Route        string                `json:"route,omitempty"`
	Tags         []string              `json:"tags,omitempty"`
	Entities     []memory.EntityRef    `json:"entities,omitempty"`
	Metadata     map[string]string     `json:"metadata,omitempty"`
	Steps        []EpisodeStepSpec     `json:"steps"`
	Context      []memory.ContextBlock `json:"context,omitempty"`
}

type EpisodeStepSpec struct {
	ToolName      string               `json:"tool_name"`
	OutputKind    string               `json:"output_kind"`
	Status        core.ExecutionStatus `json:"status,omitempty"`
	OutputSummary string               `json:"output_summary"`
	FixSummary    string               `json:"fix_summary,omitempty"`
	Error         *core.ErrorInfo      `json:"error,omitempty"`
	Input         map[string]any       `json:"input,omitempty"`
	Payload       map[string]any       `json:"payload,omitempty"`
}

type QuerySpec struct {
	Query            string               `json:"query"`
	TargetScenario   string               `json:"target_scenario,omitempty"`
	TargetIntent     string               `json:"target_intent,omitempty"`
	TargetEpisodeIDs []string             `json:"target_episode_ids"`
	Entity           string               `json:"entity,omitempty"`
	Status           memory.EpisodeStatus `json:"status,omitempty"`
	Tags             []string             `json:"tags,omitempty"`
	Notes            string               `json:"notes,omitempty"`
}

func (bundle SpecBundle) Validate() error {
	if strings.TrimSpace(bundle.SchemaVersion) == "" {
		return fmt.Errorf("schema_version is required")
	}
	episodeIDs := make(map[string]struct{}, len(bundle.Episodes))
	for index, episode := range bundle.Episodes {
		if err := episode.Validate(); err != nil {
			return fmt.Errorf("episodes[%d]: %w", index, err)
		}
		if _, exists := episodeIDs[episode.EpisodeID]; exists {
			return fmt.Errorf("episodes[%d]: duplicate episode_id %q", index, episode.EpisodeID)
		}
		episodeIDs[episode.EpisodeID] = struct{}{}
	}
	for index, query := range bundle.Queries {
		if err := query.Validate(episodeIDs); err != nil {
			return fmt.Errorf("queries[%d]: %w", index, err)
		}
	}
	return nil
}

func LoadSpecBundle(path string) (SpecBundle, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return SpecBundle{}, err
	}

	var bundle SpecBundle
	if err := json.Unmarshal(raw, &bundle); err != nil {
		return SpecBundle{}, err
	}
	normalizeSpecBundle(&bundle)
	if err := bundle.Validate(); err != nil {
		return SpecBundle{}, err
	}
	return bundle, nil
}

func (spec EpisodeSpec) Validate() error {
	if strings.TrimSpace(spec.EpisodeID) == "" {
		return fmt.Errorf("episode_id is required")
	}
	if strings.TrimSpace(spec.ClusterID) == "" {
		return fmt.Errorf("cluster_id is required")
	}
	if strings.TrimSpace(spec.Scenario) == "" {
		return fmt.Errorf("scenario is required")
	}
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	if strings.TrimSpace(spec.Intent) == "" {
		return fmt.Errorf("intent is required")
	}
	if len(spec.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}
	for index, step := range spec.Steps {
		if err := step.Validate(); err != nil {
			return fmt.Errorf("steps[%d]: %w", index, err)
		}
	}
	return nil
}

func (spec QuerySpec) Validate(episodeIDs map[string]struct{}) error {
	if strings.TrimSpace(spec.Query) == "" {
		return fmt.Errorf("query is required")
	}
	if len(spec.TargetEpisodeIDs) == 0 {
		return fmt.Errorf("target_episode_ids is required")
	}
	for index, targetEpisodeID := range spec.TargetEpisodeIDs {
		trimmed := strings.TrimSpace(targetEpisodeID)
		if trimmed == "" {
			return fmt.Errorf("target_episode_ids[%d] is required", index)
		}
		if _, exists := episodeIDs[trimmed]; !exists {
			return fmt.Errorf("target_episode_ids[%d]=%q does not match any episode_id", index, trimmed)
		}
	}
	return nil
}

func normalizeSpecBundle(bundle *SpecBundle) {
	ensureEpisodeIDs(bundle)
	ensureClusterIDs(bundle)
	ensureQueryTargetEpisodeIDs(bundle)
}

func ensureEpisodeIDs(bundle *SpecBundle) {
	usedIDs := make(map[string]struct{}, len(bundle.Episodes))
	for index := range bundle.Episodes {
		episodeID := strings.TrimSpace(bundle.Episodes[index].EpisodeID)
		if episodeID == "" {
			episodeID = makeEpisodeID(bundle.Episodes[index].Scenario, index+1, usedIDs)
			bundle.Episodes[index].EpisodeID = episodeID
		}
		if _, exists := usedIDs[episodeID]; exists {
			episodeID = makeEpisodeID(episodeID, index+1, usedIDs)
			bundle.Episodes[index].EpisodeID = episodeID
		}
		usedIDs[episodeID] = struct{}{}
	}
}

func ensureClusterIDs(bundle *SpecBundle) {
	for index := range bundle.Episodes {
		clusterID := strings.TrimSpace(bundle.Episodes[index].ClusterID)
		if clusterID != "" {
			bundle.Episodes[index].ClusterID = clusterID
			continue
		}
		bundle.Episodes[index].ClusterID = deriveClusterID(bundle.Episodes[index], index+1)
	}
}

func ensureQueryTargetEpisodeIDs(bundle *SpecBundle) {
	episodeByID := make(map[string]EpisodeSpec, len(bundle.Episodes))
	for _, episode := range bundle.Episodes {
		episodeByID[episode.EpisodeID] = episode
	}

	for index := range bundle.Queries {
		query := &bundle.Queries[index]
		if len(query.TargetEpisodeIDs) == 0 {
			query.TargetEpisodeIDs = resolveQueryTargetEpisodeIDs(*query, bundle.Episodes)
		}
		if len(query.TargetEpisodeIDs) == 0 {
			continue
		}
		targetEpisode, exists := episodeByID[query.TargetEpisodeIDs[0]]
		if !exists {
			continue
		}
		if strings.TrimSpace(query.TargetScenario) == "" {
			query.TargetScenario = targetEpisode.Scenario
		}
		if strings.TrimSpace(query.TargetIntent) == "" {
			query.TargetIntent = targetEpisode.Intent
		}
	}
}

func resolveQueryTargetEpisodeIDs(query QuerySpec, episodes []EpisodeSpec) []string {
	type candidate struct {
		score     int
		episodeID string
	}

	queryTags := make(map[string]struct{}, len(query.Tags))
	for _, tag := range query.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			queryTags[tag] = struct{}{}
		}
	}

	candidates := make([]candidate, 0, len(episodes))
	for _, episode := range episodes {
		if strings.TrimSpace(query.TargetScenario) != "" && episode.Scenario != query.TargetScenario {
			continue
		}
		if strings.TrimSpace(query.TargetIntent) != "" && episode.Intent != query.TargetIntent {
			continue
		}

		score := 0
		if query.Status != "" && episode.Status == query.Status {
			score += 3
		}
		if query.Entity != "" && episodeMatchesEntity(episode, query.Entity) {
			score += 5
		}
		for _, tag := range episode.Tags {
			if _, exists := queryTags[strings.TrimSpace(tag)]; exists {
				score++
			}
		}
		candidates = append(candidates, candidate{
			score:     score,
			episodeID: episode.EpisodeID,
		})
	}

	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].episodeID < candidates[j].episodeID
	})
	return []string{candidates[0].episodeID}
}

func deriveClusterID(spec EpisodeSpec, index int) string {
	scenarioSlug := slugifyToken(spec.Scenario)
	episodeID := strings.TrimPrefix(strings.TrimSpace(spec.EpisodeID), "ep_")
	if scenarioSlug != "" && episodeID != "" {
		marker := scenarioSlug + "_c"
		if start := strings.Index(episodeID, marker); start >= 0 {
			clusterID := consumeClusterSlug(episodeID[start:])
			if clusterID != "" {
				return clusterID
			}
		}
	}
	if scenarioSlug == "" {
		scenarioSlug = "episode"
	}
	return fmt.Sprintf("%s_singleton_%03d", scenarioSlug, index)
}

func consumeClusterSlug(value string) string {
	parts := strings.Split(value, "_")
	if len(parts) == 0 {
		return ""
	}
	clusterParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		clusterParts = append(clusterParts, part)
		if isClusterCounterToken(part) {
			return strings.Join(clusterParts, "_")
		}
	}
	return ""
}

func isClusterCounterToken(value string) bool {
	if len(value) < 2 || value[0] != 'c' {
		return false
	}
	for _, r := range value[1:] {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func episodeMatchesEntity(episode EpisodeSpec, entity string) bool {
	for _, ref := range episode.Entities {
		if strings.TrimSpace(ref.Name) == entity {
			return true
		}
	}
	for _, value := range episode.Metadata {
		if strings.TrimSpace(value) == entity {
			return true
		}
	}
	return strings.Contains(episode.Goal, entity) || strings.Contains(episode.Summary, entity)
}

func makeEpisodeID(seed string, index int, usedIDs map[string]struct{}) string {
	base := slugifyToken(seed)
	if base == "" {
		base = "episode"
	}
	candidate := fmt.Sprintf("ep_%s_%03d", base, index)
	suffix := index
	for {
		if _, exists := usedIDs[candidate]; !exists {
			return candidate
		}
		suffix++
		candidate = fmt.Sprintf("ep_%s_%03d", base, suffix)
	}
}

func slugifyToken(value string) string {
	var builder strings.Builder
	previousWasSep := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			previousWasSep = false
			continue
		}
		if previousWasSep {
			continue
		}
		builder.WriteRune('_')
		previousWasSep = true
	}
	return strings.Trim(builder.String(), "_")
}

func (step EpisodeStepSpec) Validate() error {
	if strings.TrimSpace(step.ToolName) == "" {
		return fmt.Errorf("tool_name is required")
	}
	if strings.TrimSpace(step.OutputKind) == "" {
		return fmt.Errorf("output_kind is required")
	}
	return nil
}

func CompileEpisodeSpecs(specs []EpisodeSpec, seed int64, userID string) ([]memory.Episode, error) {
	if userID == "" {
		userID = "mock-user-001"
	}
	rng := rand.New(rand.NewSource(seed))
	now := time.Now().UTC()
	episodes := make([]memory.Episode, 0, len(specs))

	for index, spec := range specs {
		if err := spec.Validate(); err != nil {
			return nil, fmt.Errorf("compile episode spec %d: %w", index, err)
		}
		startedAt := now.Add(-time.Duration(24+index*6+rng.Intn(8)) * time.Hour)
		threadID := fmt.Sprintf("spec-media-thread-%03d", index+1)
		episodes = append(episodes, compileEpisodeSpec(rng, spec, threadID, userID, startedAt))
	}

	return episodes, nil
}

func ExampleSpecBundle() SpecBundle {
	return SpecBundle{
		SchemaVersion: SpecSchemaVersion,
		Episodes: []EpisodeSpec{
			{
				EpisodeID:    "ep_curate_trip_album_tokyo_001",
				ClusterID:    "curate_trip_album_c01",
				Scenario:     "curate_trip_album",
				Goal:         "find Tokyo trip photos from spring 2024, group them by date, and build an album",
				Intent:       "curate_album",
				Status:       memory.EpisodeStatusSucceeded,
				WriteTrigger: memory.WriteTriggerGoalResolved,
				Tags:         []string{"media", "album", "travel"},
				Entities: []memory.EntityRef{
					{Type: "location", Name: "Tokyo"},
					{Type: "album", Name: "Tokyo Spring Selects"},
				},
				Metadata: map[string]string{
					"location":     "Tokyo",
					"camera_model": "Fujifilm X-T5",
					"album_name":   "Tokyo Spring Selects",
				},
				Steps: []EpisodeStepSpec{
					{ToolName: "mock_filter_assets", OutputKind: "asset_selection", Status: core.ExecutionStatusSuccess, OutputSummary: "Selected spring travel photos captured in Tokyo."},
					{ToolName: "mock_group_assets", OutputKind: "asset_groups", Status: core.ExecutionStatusSuccess, OutputSummary: "Grouped the selected assets by date."},
					{ToolName: "mock_create_album", OutputKind: "album_record", Status: core.ExecutionStatusSuccess, OutputSummary: "Created the album Tokyo Spring Selects."},
					{ToolName: "mock_add_assets_to_album", OutputKind: "album_membership_update", Status: core.ExecutionStatusSuccess, OutputSummary: "Added the grouped travel photos to the album."},
				},
			},
			{
				EpisodeID:    "ep_cleanup_duplicates_yosemite_001",
				ClusterID:    "duplicate_cleanup_recovery_c01",
				Scenario:     "duplicate_cleanup_recovery",
				Goal:         "remove duplicate burst shots while keeping the best photo in each duplicate group",
				Intent:       "cleanup_duplicates",
				Status:       memory.EpisodeStatusRecovered,
				WriteTrigger: memory.WriteTriggerErrorRecovered,
				Tags:         []string{"media", "duplicates", "recovered"},
				Entities: []memory.EntityRef{
					{Type: "failure_mode", Name: "false_positive_duplicate"},
					{Type: "location", Name: "Yosemite"},
				},
				Metadata: map[string]string{
					"location":     "Yosemite",
					"camera_model": "Sony A7C II",
				},
				Steps: []EpisodeStepSpec{
					{ToolName: "mock_find_duplicate_assets", OutputKind: "duplicate_report", Status: core.ExecutionStatusError, OutputSummary: "The initial duplicate pass grouped unrelated burst shots together.", Error: &core.ErrorInfo{Code: "FALSE_POSITIVE_DUPLICATE", Message: "Similarity threshold was too loose."}},
					{ToolName: "mock_inspect_asset_metadata", OutputKind: "asset_metadata_report", Status: core.ExecutionStatusSuccess, OutputSummary: "Checked timestamps and exposure metadata to confirm the mismatch."},
					{ToolName: "mock_find_duplicate_assets", OutputKind: "duplicate_report", Status: core.ExecutionStatusSuccess, OutputSummary: "Reran duplicate detection with a stricter threshold.", FixSummary: "Raised similarity threshold and compared capture timestamps."},
					{ToolName: "mock_bulk_archive_assets", OutputKind: "bulk_archive_update", Status: core.ExecutionStatusSuccess, OutputSummary: "Archived the duplicate photos and kept the sharpest versions."},
				},
			},
		},
		Queries: []QuerySpec{
			{
				Query:            "How did I organize my Tokyo spring trip photos into an album last time?",
				TargetScenario:   "curate_trip_album",
				TargetIntent:     "curate_album",
				TargetEpisodeIDs: []string{"ep_curate_trip_album_tokyo_001"},
				Entity:           "Tokyo",
				Tags:             []string{"travel", "album"},
			},
			{
				Query:            "What fix worked for duplicate burst shots that were falsely grouped together?",
				TargetScenario:   "duplicate_cleanup_recovery",
				TargetIntent:     "cleanup_duplicates",
				TargetEpisodeIDs: []string{"ep_cleanup_duplicates_yosemite_001"},
				Status:           memory.EpisodeStatusRecovered,
				Tags:             []string{"duplicates", "recovered"},
			},
		},
	}
}

func compileEpisodeSpec(rng *rand.Rand, spec EpisodeSpec, threadID, userID string, startedAt time.Time) memory.Episode {
	status := spec.Status
	if status == "" {
		status = memory.EpisodeStatusSucceeded
	}
	writeTrigger := spec.WriteTrigger
	if writeTrigger == "" {
		writeTrigger = defaultWriteTrigger(status)
	}

	steps := make([]memory.ToolTraceStep, 0, len(spec.Steps))
	refs := make([]string, 0, len(spec.Steps))
	toolNames := make([]string, 0, len(spec.Steps))
	cursor := startedAt
	metadataPayload := metadataToAnyMap(spec.Metadata)
	if strings.TrimSpace(spec.ClusterID) != "" {
		if metadataPayload == nil {
			metadataPayload = make(map[string]any, 1)
		}
		metadataPayload["cluster_id"] = spec.ClusterID
	}

	for index, step := range spec.Steps {
		stepStatus := step.Status
		if stepStatus == "" {
			stepStatus = core.ExecutionStatusSuccess
		}

		stepStart := cursor
		stepEnd := stepStart.Add(time.Duration(4+rng.Intn(9)) * time.Second)
		cursor = stepEnd.Add(time.Duration(1+rng.Intn(4)) * time.Second)

		toolID := core.ToolIdentity{
			Name:        step.ToolName,
			ExecutionID: uuid.NewString(),
		}

		refID := ""
		if stepStatus == core.ExecutionStatusSuccess {
			refID = fmt.Sprintf("ref.%s.%s.%s", step.ToolName, step.OutputKind, strings.ReplaceAll(uuid.NewString(), "-", ""))
			refs = append(refs, refID)
		}

		payload := step.Payload
		if len(payload) == 0 {
			payload = metadataPayload
		}
		input := step.Input
		if len(input) == 0 {
			input = metadataPayload
		}

		sideEvent := &core.SideChannelEvent{
			Type:      "tool_execution",
			Timestamp: stepEnd.UnixMilli(),
			Tool:      toolID,
			Execution: core.ExecutionInfo{
				Status:     stepStatus,
				Message:    step.OutputSummary,
				Error:      step.Error,
				Parameters: input,
				Duration:   stepEnd.Sub(stepStart).Milliseconds(),
			},
			Data: &core.DataPayload{
				RefID:       refID,
				PayloadType: step.OutputKind,
				Payload:     payload,
			},
		}

		steps = append(steps, memory.ToolTraceStep{
			Index:         index,
			Tool:          toolID,
			Operation:     step.ToolName,
			Input:         input,
			OutputSummary: step.OutputSummary,
			Status:        stepStatus,
			Error:         step.Error,
			FixSummary:    step.FixSummary,
			StartedAt:     stepStart,
			FinishedAt:    stepEnd,
			SideEvent:     sideEvent,
		})
		toolNames = append(toolNames, step.ToolName)
	}

	summary := strings.TrimSpace(spec.Summary)
	if summary == "" {
		summary = fmt.Sprintf("%s. Tool flow: %s.", spec.Goal, strings.Join(toolNames, " -> "))
	}

	contextBlocks := make([]memory.ContextBlock, 0, 2+len(spec.Context))
	contextBlocks = append(contextBlocks,
		memory.ContextBlock{ID: uuid.NewString(), Kind: "summary", Text: summary, Weight: 1.0},
		memory.ContextBlock{ID: uuid.NewString(), Kind: "tool_trace", Text: strings.Join(toolNames, " -> "), Weight: 0.8},
	)
	for _, block := range spec.Context {
		blockCopy := block
		if strings.TrimSpace(blockCopy.ID) == "" {
			blockCopy.ID = uuid.NewString()
		}
		contextBlocks = append(contextBlocks, blockCopy)
	}

	episode := memory.Episode{
		ID:            spec.EpisodeID,
		ThreadID:      threadID,
		UserID:        userID,
		AgentName:     firstNonEmpty(spec.AgentName, "Mock Media Memory Agent"),
		Scenario:      spec.Scenario,
		Goal:          spec.Goal,
		Intent:        spec.Intent,
		Summary:       summary,
		Workspace:     firstNonEmpty(spec.Workspace, "media-memory-lab"),
		Route:         firstNonEmpty(spec.Route, "/mock/media/assistant"),
		Status:        status,
		WriteTrigger:  writeTrigger,
		StartedAt:     startedAt,
		EndedAt:       cursor,
		Tags:          append([]string{}, spec.Tags...),
		Entities:      append([]memory.EntityRef{}, spec.Entities...),
		Refs:          refs,
		ToolTrace:     steps,
		ContextBlocks: contextBlocks,
		Metadata:      cloneStringMap(spec.Metadata),
	}
	if strings.TrimSpace(spec.ClusterID) != "" {
		if episode.Metadata == nil {
			episode.Metadata = make(map[string]string, 1)
		}
		episode.Metadata["cluster_id"] = spec.ClusterID
	}
	episode.RetrievalText = episode.BuildRetrievalText()
	return episode
}

func defaultWriteTrigger(status memory.EpisodeStatus) memory.WriteTrigger {
	switch status {
	case memory.EpisodeStatusFailed:
		return memory.WriteTriggerErrorCaptured
	case memory.EpisodeStatusRecovered:
		return memory.WriteTriggerErrorRecovered
	default:
		return memory.WriteTriggerGoalResolved
	}
}

func metadataToAnyMap(metadata map[string]string) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
