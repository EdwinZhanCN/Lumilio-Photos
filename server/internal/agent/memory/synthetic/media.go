package synthetic

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/memory"

	"github.com/google/uuid"
)

type mediaScenario struct {
	Name         string
	Intent       string
	Goal         string
	Status       memory.EpisodeStatus
	WriteTrigger memory.WriteTrigger
	Tags         []string
	EntityRefs   []memory.EntityRef
	Steps        []mediaScenarioStep
}

type mediaScenarioStep struct {
	ToolName      string
	OutputKind    string
	Status        core.ExecutionStatus
	OutputSummary string
	FixSummary    string
	Error         *core.ErrorInfo
}

func GenerateMediaEpisodes(count int, seed int64, userID string) []memory.Episode {
	if count <= 0 {
		return nil
	}
	if userID == "" {
		userID = "mock-user-001"
	}

	rng := rand.New(rand.NewSource(seed))
	now := time.Now().UTC()
	scenarios := defaultMediaScenarios()
	episodes := make([]memory.Episode, 0, count)

	for i := 0; i < count; i++ {
		scenario := scenarios[i%len(scenarios)]
		baseTime := now.Add(-time.Duration(12+rng.Intn(360)) * time.Hour)
		threadID := fmt.Sprintf("mock-media-thread-%03d", i+1)
		episodes = append(episodes, buildMediaEpisode(rng, scenario, threadID, userID, baseTime))
	}

	return episodes
}

func buildMediaEpisode(rng *rand.Rand, scenario mediaScenario, threadID, userID string, startedAt time.Time) memory.Episode {
	projectLocation := pick(rng, []string{"Tokyo", "Kyoto", "Yosemite", "New York", "Iceland"})
	cameraModel := pick(rng, []string{"Fujifilm X-T5", "Sony A7C II", "Canon R6 Mark II", "iPhone 16 Pro"})
	albumName := pick(rng, []string{"Spring Trip Picks", "Family Highlights", "RAW Keepers", "Weekend Favorites"})

	steps := make([]memory.ToolTraceStep, 0, len(scenario.Steps))
	refs := make([]string, 0, len(scenario.Steps))
	toolNames := make([]string, 0, len(scenario.Steps))
	cursor := startedAt

	for idx, step := range scenario.Steps {
		stepStart := cursor
		stepEnd := stepStart.Add(time.Duration(4+rng.Intn(10)) * time.Second)
		cursor = stepEnd.Add(time.Duration(1+rng.Intn(4)) * time.Second)

		toolID := core.ToolIdentity{
			Name:        step.ToolName,
			ExecutionID: uuid.NewString(),
		}

		refID := ""
		if step.Status == core.ExecutionStatusSuccess {
			refID = fmt.Sprintf("ref.%s.%s.%s", step.ToolName, step.OutputKind, strings.ReplaceAll(uuid.NewString(), "-", ""))
			refs = append(refs, refID)
		}

		sideEvent := &core.SideChannelEvent{
			Type:      "tool_execution",
			Timestamp: stepEnd.UnixMilli(),
			Tool:      toolID,
			Execution: core.ExecutionInfo{
				Status:   step.Status,
				Message:  step.OutputSummary,
				Error:    step.Error,
				Duration: stepEnd.Sub(stepStart).Milliseconds(),
			},
			Data: &core.DataPayload{
				RefID:       refID,
				PayloadType: step.OutputKind,
				Payload: map[string]any{
					"location":     projectLocation,
					"camera_model": cameraModel,
					"album_name":   albumName,
				},
			},
		}

		steps = append(steps, memory.ToolTraceStep{
			Index:         idx,
			Tool:          toolID,
			Operation:     step.ToolName,
			Input:         map[string]any{"location": projectLocation, "camera_model": cameraModel, "album_name": albumName},
			OutputSummary: step.OutputSummary,
			Status:        step.Status,
			Error:         step.Error,
			FixSummary:    step.FixSummary,
			StartedAt:     stepStart,
			FinishedAt:    stepEnd,
			SideEvent:     sideEvent,
		})
		toolNames = append(toolNames, step.ToolName)
	}

	summary := fmt.Sprintf("%s. Location=%s. Camera=%s. Tool flow: %s.", scenario.Goal, projectLocation, cameraModel, strings.Join(toolNames, " -> "))
	switch scenario.Status {
	case memory.EpisodeStatusRecovered:
		summary += " The agent recovered from a media-management error and finished the task."
	case memory.EpisodeStatusFailed:
		summary += " The task ended in a captured failure for later recall."
	}

	entities := append([]memory.EntityRef{
		{Type: "location", Name: projectLocation},
		{Type: "camera_model", Name: cameraModel},
		{Type: "album", Name: albumName},
	}, scenario.EntityRefs...)

	episode := memory.Episode{
		ID:           uuid.NewString(),
		ThreadID:     threadID,
		UserID:       userID,
		AgentName:    "Mock Media Memory Agent",
		Scenario:     scenario.Name,
		Goal:         scenario.Goal,
		Intent:       scenario.Intent,
		Summary:      summary,
		Workspace:    "media-memory-lab",
		Route:        "/mock/media/assistant",
		Status:       scenario.Status,
		WriteTrigger: scenario.WriteTrigger,
		StartedAt:    startedAt,
		EndedAt:      cursor,
		Tags:         append([]string{}, scenario.Tags...),
		Entities:     entities,
		Refs:         refs,
		ToolTrace:    steps,
		ContextBlocks: []memory.ContextBlock{
			{ID: uuid.NewString(), Kind: "summary", Text: summary, Weight: 1.0},
			{ID: uuid.NewString(), Kind: "tool_trace", Text: strings.Join(toolNames, " -> "), Weight: 0.8},
		},
		Metadata: map[string]string{
			"location":     projectLocation,
			"camera_model": cameraModel,
			"album_name":   albumName,
		},
	}
	episode.RetrievalText = episode.BuildRetrievalText()
	return episode
}

func defaultMediaScenarios() []mediaScenario {
	return []mediaScenario{
		{
			Name:         "curate_trip_album",
			Intent:       "curate_album",
			Goal:         "find travel photos from a recent trip, group them by date, and create an album",
			Status:       memory.EpisodeStatusSucceeded,
			WriteTrigger: memory.WriteTriggerGoalResolved,
			Tags:         []string{"media", "album", "travel", "success"},
			EntityRefs: []memory.EntityRef{
				{Type: "task", Name: "trip_curation"},
			},
			Steps: []mediaScenarioStep{
				{ToolName: "mock_filter_assets", OutputKind: "asset_selection", Status: core.ExecutionStatusSuccess, OutputSummary: "Selected travel photos matching the user's constraints."},
				{ToolName: "mock_group_assets", OutputKind: "asset_groups", Status: core.ExecutionStatusSuccess, OutputSummary: "Grouped the selected assets by date."},
				{ToolName: "mock_create_album", OutputKind: "album_record", Status: core.ExecutionStatusSuccess, OutputSummary: "Created a new album for the trip highlights."},
				{ToolName: "mock_add_assets_to_album", OutputKind: "album_membership_update", Status: core.ExecutionStatusSuccess, OutputSummary: "Added the selected assets to the new album."},
				{ToolName: "mock_summarize_selection", OutputKind: "selection_summary", Status: core.ExecutionStatusSuccess, OutputSummary: "Summarized the curated album for the user."},
			},
		},
		{
			Name:         "bulk_like_favorites",
			Intent:       "bulk_preference_update",
			Goal:         "find highly rated photos from last spring and mark them as liked",
			Status:       memory.EpisodeStatusSucceeded,
			WriteTrigger: memory.WriteTriggerGoalResolved,
			Tags:         []string{"media", "likes", "bulk-action"},
			EntityRefs: []memory.EntityRef{
				{Type: "task", Name: "favorite_curation"},
			},
			Steps: []mediaScenarioStep{
				{ToolName: "mock_filter_assets", OutputKind: "asset_selection", Status: core.ExecutionStatusSuccess, OutputSummary: "Selected 5-star spring photos that were not yet liked."},
				{ToolName: "mock_bulk_like_assets", OutputKind: "bulk_like_update", Status: core.ExecutionStatusSuccess, OutputSummary: "Applied a bulk like action to the selection."},
				{ToolName: "mock_summarize_selection", OutputKind: "selection_summary", Status: core.ExecutionStatusSuccess, OutputSummary: "Summarized how many photos were updated."},
			},
		},
		{
			Name:         "duplicate_cleanup_recovery",
			Intent:       "cleanup_duplicates",
			Goal:         "remove duplicate photos while keeping the best version of each group",
			Status:       memory.EpisodeStatusRecovered,
			WriteTrigger: memory.WriteTriggerErrorRecovered,
			Tags:         []string{"media", "duplicates", "recovered"},
			EntityRefs: []memory.EntityRef{
				{Type: "failure_mode", Name: "false_positive_duplicate"},
			},
			Steps: []mediaScenarioStep{
				{ToolName: "mock_find_duplicate_assets", OutputKind: "duplicate_report", Status: core.ExecutionStatusError, OutputSummary: "The first duplicate pass produced suspicious matches.", Error: &core.ErrorInfo{Code: "FALSE_POSITIVE_DUPLICATE", Message: "Similarity threshold grouped unrelated burst shots."}},
				{ToolName: "mock_inspect_asset_metadata", OutputKind: "asset_metadata_report", Status: core.ExecutionStatusSuccess, OutputSummary: "Metadata inspection confirmed the duplicate threshold was too loose."},
				{ToolName: "mock_find_duplicate_assets", OutputKind: "duplicate_report", Status: core.ExecutionStatusSuccess, OutputSummary: "Regenerated duplicate groups with a stricter threshold.", FixSummary: "Raised duplicate threshold and compared timestamps."},
				{ToolName: "mock_bulk_archive_assets", OutputKind: "bulk_archive_update", Status: core.ExecutionStatusSuccess, OutputSummary: "Archived the duplicate assets while keeping the best versions."},
				{ToolName: "mock_summarize_selection", OutputKind: "selection_summary", Status: core.ExecutionStatusSuccess, OutputSummary: "Summarized the duplicate cleanup outcome."},
			},
		},
		{
			Name:         "metadata_drilldown",
			Intent:       "inspect_metadata",
			Goal:         "inspect metadata for a selected camera and summarize the matching assets",
			Status:       memory.EpisodeStatusSucceeded,
			WriteTrigger: memory.WriteTriggerGoalResolved,
			Tags:         []string{"media", "metadata", "camera"},
			EntityRefs: []memory.EntityRef{
				{Type: "task", Name: "metadata_drilldown"},
			},
			Steps: []mediaScenarioStep{
				{ToolName: "mock_filter_assets", OutputKind: "asset_selection", Status: core.ExecutionStatusSuccess, OutputSummary: "Filtered photos captured with the selected camera model."},
				{ToolName: "mock_inspect_asset_metadata", OutputKind: "asset_metadata_report", Status: core.ExecutionStatusSuccess, OutputSummary: "Inspected EXIF and capture metadata for the selection."},
				{ToolName: "mock_group_assets", OutputKind: "asset_groups", Status: core.ExecutionStatusSuccess, OutputSummary: "Grouped the assets by location for easier review."},
				{ToolName: "mock_summarize_selection", OutputKind: "selection_summary", Status: core.ExecutionStatusSuccess, OutputSummary: "Summarized the metadata highlights for the user."},
			},
		},
	}
}

func pick(rng *rand.Rand, values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[rng.Intn(len(values))]
}
