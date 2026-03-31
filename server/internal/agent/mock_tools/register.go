package mocktools

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"server/internal/agent/core"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

type MockToolInput struct {
	Query       string   `json:"query,omitempty" jsonschema:"description=Natural-language request for the media assistant, such as 'show my liked RAW photos from last spring'."`
	AssetIDs    []string `json:"asset_ids,omitempty" jsonschema:"description=Optional synthetic asset identifiers for album or bulk actions."`
	AlbumName   string   `json:"album_name,omitempty" jsonschema:"description=Album name used for album creation or album membership actions."`
	GroupBy     string   `json:"group_by,omitempty" jsonschema:"description=Grouping mode such as date, type, location, or album."`
	Limit       int      `json:"limit,omitempty" jsonschema:"description=Maximum number of synthetic assets to return or mutate."`
	ForceError  bool     `json:"force_error,omitempty" jsonschema:"description=When true, the tool emits a synthetic failure instead of a success artifact."`
	ActionLabel string   `json:"action_label,omitempty" jsonschema:"description=Optional action label such as keep_best, archive_duplicates, or favorite."`
}

type MockArtifact struct {
	Tool               string            `json:"tool"`
	Category           string            `json:"category"`
	Query              string            `json:"query,omitempty"`
	AlbumName          string            `json:"album_name,omitempty"`
	GroupBy            string            `json:"group_by,omitempty"`
	AssetCount         int               `json:"asset_count"`
	AssetIDs           []string          `json:"asset_ids,omitempty"`
	CoverAssetID       string            `json:"cover_asset_id,omitempty"`
	TimeRange          string            `json:"time_range,omitempty"`
	MetadataHighlights map[string]string `json:"metadata_highlights,omitempty"`
	Outcome            string            `json:"outcome"`
	Summary            string            `json:"summary"`
	Tags               []string          `json:"tags,omitempty"`
	SuggestedNextTools []string          `json:"suggested_next_tools,omitempty"`
}

type MockToolOutput struct {
	Message  string        `json:"message"`
	RefID    string        `json:"ref_id,omitempty"`
	Artifact *MockArtifact `json:"artifact,omitempty"`
}

func RegisterAll() {
	for _, definition := range Catalog() {
		registerDefinition(definition)
	}
}

func registerDefinition(definition Definition) {
	info := &schema.ToolInfo{
		Name: definition.Name,
		Desc: definition.Description,
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		t, err := utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *MockToolInput) (*MockToolOutput, error) {
			startedAt := time.Now()
			executionID := fmt.Sprintf("%d", startedAt.UnixNano())

			sendMockEvent(deps, definition, executionID, startedAt, core.ExecutionStatusPending, "Queued mock tool execution...", input, nil, nil, 0)
			sendMockEvent(deps, definition, executionID, time.Now(), core.ExecutionStatusRunning, "Synthesizing mock output...", input, nil, nil, 0)

			if input != nil && input.ForceError {
				errInfo := &core.ErrorInfo{
					Code:    "MOCK_TOOL_ERROR",
					Message: fmt.Sprintf("%s produced a simulated media-management failure for research purposes", definition.Name),
					Details: map[string]any{"query": input.Query, "album_name": input.AlbumName},
				}
				sendMockEvent(
					deps,
					definition,
					executionID,
					time.Now(),
					core.ExecutionStatusError,
					"Mock failure emitted",
					input,
					nil,
					errInfo,
					time.Since(startedAt).Milliseconds(),
				)
				return &MockToolOutput{
					Message: errInfo.Message,
				}, nil
			}

			artifact := buildArtifact(definition, input)
			refID := ""
			if deps.ReferenceManager != nil {
				refID = deps.ReferenceManager.Store(ctx, artifact, core.ReferenceDescriptor{
					SourceTool:  definition.Name,
					Kind:        definition.OutputKind,
					Description: artifact.Summary,
				})
			}

			sendMockEvent(
				deps,
				definition,
				executionID,
				time.Now(),
				core.ExecutionStatusSuccess,
				artifact.Summary,
				input,
				&core.DataPayload{
					RefID:       refID,
					PayloadType: "mock_tools.MockArtifact",
					Payload:     artifact,
				},
				nil,
				time.Since(startedAt).Milliseconds(),
			)

			return &MockToolOutput{
				Message:  artifact.Summary,
				RefID:    refID,
				Artifact: artifact,
			}, nil
		})
		if err != nil {
			return nil, err
		}
		return t, nil
	})
}

func buildArtifact(definition Definition, input *MockToolInput) *MockArtifact {
	query := "show my recent favorite travel photos"
	albumName := ""
	groupBy := "date"
	limit := 24
	actionLabel := "default"
	var explicitAssetIDs []string
	if input != nil {
		if input.Query != "" {
			query = input.Query
		}
		if input.AlbumName != "" {
			albumName = input.AlbumName
		}
		if input.GroupBy != "" {
			groupBy = input.GroupBy
		}
		if input.Limit > 0 {
			limit = input.Limit
		}
		if input.ActionLabel != "" {
			actionLabel = input.ActionLabel
		}
		if len(input.AssetIDs) > 0 {
			explicitAssetIDs = append(explicitAssetIDs, input.AssetIDs...)
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	assetIDs := explicitAssetIDs
	assetCount := len(assetIDs)
	if assetCount == 0 {
		assetCount = 4 + rng.Intn(max(limit, 5))
		if assetCount > limit {
			assetCount = limit
		}
		if assetCount <= 0 {
			assetCount = 1
		}
		assetIDs = make([]string, 0, assetCount)
		for i := 0; i < assetCount; i++ {
			assetIDs = append(assetIDs, fmt.Sprintf("mock-asset-%04d", 1000+rng.Intn(9000)))
		}
	}
	coverAssetID := ""
	if len(assetIDs) > 0 {
		coverAssetID = assetIDs[0]
	}

	return &MockArtifact{
		Tool:               definition.Name,
		Category:           definition.Category,
		Query:              query,
		AlbumName:          albumName,
		GroupBy:            groupBy,
		AssetCount:         assetCount,
		AssetIDs:           assetIDs,
		CoverAssetID:       coverAssetID,
		TimeRange:          randomTimeRange(rng),
		MetadataHighlights: randomHighlights(rng, definition.Name, groupBy, actionLabel),
		Outcome:            "success",
		Summary:            summarizeArtifact(definition, query, albumName, groupBy, assetCount, actionLabel),
		Tags:               definition.Tags,
		SuggestedNextTools: definition.SuggestedNextTools,
	}
}

func sendMockEvent(
	deps *core.ToolDependencies,
	definition Definition,
	executionID string,
	timestamp time.Time,
	status core.ExecutionStatus,
	message string,
	input *MockToolInput,
	payload *core.DataPayload,
	errInfo *core.ErrorInfo,
	duration int64,
) {
	if deps == nil || deps.SideChannel == nil {
		return
	}

	event := &core.SideChannelEvent{
		Type:      "tool_execution",
		Timestamp: timestamp.UnixMilli(),
		Tool: core.ToolIdentity{
			Name:        definition.Name,
			ExecutionID: executionID,
		},
		Execution: core.ExecutionInfo{
			Status:     status,
			Message:    message,
			Error:      errInfo,
			Parameters: input,
			Duration:   duration,
		},
		Data: payload,
		Metadata: map[string]any{
			"category": definition.Category,
			"tags":     strings.Join(definition.Tags, ","),
		},
	}

	deps.SideChannel <- event
}

func summarizeArtifact(definition Definition, query, albumName, groupBy string, assetCount int, actionLabel string) string {
	switch definition.Name {
	case "mock_filter_assets":
		return fmt.Sprintf("Selected %d synthetic assets for query %q.", assetCount, query)
	case "mock_group_assets":
		return fmt.Sprintf("Grouped %d synthetic assets by %s.", assetCount, groupBy)
	case "mock_inspect_asset_metadata":
		return fmt.Sprintf("Inspected metadata for %d synthetic assets related to %q.", assetCount, query)
	case "mock_find_duplicate_assets":
		return fmt.Sprintf("Found %d synthetic duplicate candidates using policy %s.", assetCount, actionLabel)
	case "mock_bulk_like_assets":
		return fmt.Sprintf("Applied a bulk-like style action to %d synthetic assets.", assetCount)
	case "mock_bulk_archive_assets":
		return fmt.Sprintf("Archived %d synthetic assets after review.", assetCount)
	case "mock_create_album":
		if albumName == "" {
			albumName = "Untitled Mock Album"
		}
		return fmt.Sprintf("Created synthetic album %q for %d assets.", albumName, assetCount)
	case "mock_add_assets_to_album":
		if albumName == "" {
			albumName = "Untitled Mock Album"
		}
		return fmt.Sprintf("Added %d synthetic assets to album %q.", assetCount, albumName)
	case "mock_summarize_selection":
		return fmt.Sprintf("Summarized a synthetic media selection of %d assets.", assetCount)
	default:
		return fmt.Sprintf("%s produced a synthetic %s artifact.", definition.Name, definition.OutputKind)
	}
}

func randomTimeRange(rng *rand.Rand) string {
	options := []string{
		"last weekend",
		"spring 2024",
		"summer vacation 2023",
		"last 30 days",
		"winter holidays 2022",
	}
	return options[rng.Intn(len(options))]
}

func randomHighlights(rng *rand.Rand, toolName, groupBy, actionLabel string) map[string]string {
	highlights := map[string]string{
		"camera_model": randomChoice(rng, []string{"Fujifilm X-T5", "Sony A7C II", "iPhone 16 Pro", "Canon R6 Mark II"}),
		"location":     randomChoice(rng, []string{"Tokyo", "Kyoto", "New York", "Yosemite"}),
		"group_by":     groupBy,
	}
	if actionLabel != "" && actionLabel != "default" {
		highlights["policy"] = actionLabel
	}
	if toolName == "mock_find_duplicate_assets" {
		highlights["similarity_threshold"] = "0.94"
	}
	return highlights
}

func randomChoice(rng *rand.Rand, options []string) string {
	if len(options) == 0 {
		return ""
	}
	return options[rng.Intn(len(options))]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
