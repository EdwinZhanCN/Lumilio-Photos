package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5/pgtype"
)

// AssetFilterInput defines how the LLM calls filter_assets.
type AssetFilterInput struct {
	DateFrom string   `json:"date_from,omitempty" jsonschema:"description=Start date in YYYY-MM-DD format"`
	DateTo   string   `json:"date_to,omitempty" jsonschema:"description=End date in YYYY-MM-DD format"`
	Type     string   `json:"type,omitempty" jsonschema:"enum=PHOTO,enum=VIDEO,enum=AUDIO,description=Asset type"`
	Filename string   `json:"filename,omitempty" jsonschema:"description=Filename substring to search for"`
	Raw      *bool    `json:"raw,omitempty" jsonschema:"description=Filter for RAW photos only"`
	Rating   *int     `json:"rating,omitempty" jsonschema:"description=Filter by exact rating (0-5)"`
	Liked    *bool    `json:"liked,omitempty" jsonschema:"description=Filter for liked/favorited assets"`
	Place    string   `json:"place,omitempty" jsonschema:"description=Place name to match against the library's location clusters (e.g. Kyoto, Tokyo Tower)"`
	Camera   string   `json:"camera,omitempty" jsonschema:"description=Camera model substring (e.g. Nikon Z8)"`
	Lens     string   `json:"lens,omitempty" jsonschema:"description=Lens model substring"`
	AlbumID  *int     `json:"album_id,omitempty" jsonschema:"description=Filter to assets in this album id"`
	TagNames []string `json:"tag_names,omitempty" jsonschema:"description=Tag names to filter by (AND semantics — assets must have all listed tags)"`
}

// RegisterFilterAssets registers the filter_assets producer: metadata
// conditions in, ref out. The matching asset ids are materialized eagerly as
// an ordered snapshot (capture time desc); only the receipt reaches the LLM.
func RegisterFilterAssets() {
	info := &schema.ToolInfo{
		Name: "filter_assets",
		Desc: "Find assets by metadata conditions (date range, type, filename, RAW, rating, liked, place, camera, lens, album, tags). " +
			"Returns a ref: a handle for the matching set. Pass the ref to other tools " +
			"(combine, describe, show, bulk_like_assets, tag_assets) to work with the set.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *AssetFilterInput) (*RefToolOutput, error) {
			start := time.Now()
			execID := newExecutionID()
			sendRunning(deps, info.Name, execID, "Filtering assets...", input)

			params, refErr := buildFilterParams(input)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			rows, err := deps.Queries.GetAssetIDsUnified(ctx, *params)
			if err != nil {
				refErr := ref.Internal("asset filter query")
				sendError(deps, info.Name, execID, start, refErr)
				return errorOutput(refErr), nil
			}

			truncated := len(rows) > ref.MaxSnapshotSize
			if truncated {
				rows = rows[:ref.MaxSnapshotSize]
			}

			snapshot := fromPgUUIDs(rows)
			summary := filterSummary(input, len(snapshot), truncated)
			r := deps.RefStore.Create(
				deps.Scope(),
				ref.Plan{Op: info.Name, Params: filterPlanParams(input)},
				filterHint(input),
				summary,
				snapshot,
				truncated,
			)
			sendSuccess(deps, info.Name, execID, start, summary, &core.DataPayload{RefID: r.ID, Count: r.Count()})
			return receiptOutput(r, summary), nil
		})
	})
}

func buildFilterParams(input *AssetFilterInput) (*repo.GetAssetIDsUnifiedParams, *ref.Error) {
	// Fetch one row past the cap so truncation is detectable.
	params := repo.GetAssetIDsUnifiedParams{Limit: ref.MaxSnapshotSize + 1}

	if input.DateFrom != "" {
		t, err := time.Parse("2006-01-02", input.DateFrom)
		if err != nil {
			return nil, ref.InvalidArgument(fmt.Sprintf("date_from %q is not YYYY-MM-DD", input.DateFrom))
		}
		params.DateFrom = pgtype.Timestamptz{Time: t, Valid: true}
	}
	if input.DateTo != "" {
		t, err := time.Parse("2006-01-02", input.DateTo)
		if err != nil {
			return nil, ref.InvalidArgument(fmt.Sprintf("date_to %q is not YYYY-MM-DD", input.DateTo))
		}
		// Inclusive end of day.
		t = t.Add(24*time.Hour - time.Nanosecond)
		params.DateTo = pgtype.Timestamptz{Time: t, Valid: true}
	}
	if input.Type != "" {
		assetType := strings.ToUpper(strings.TrimSpace(input.Type))
		switch assetType {
		case "PHOTO", "VIDEO", "AUDIO":
			params.AssetType = &assetType
		default:
			return nil, ref.InvalidArgument(fmt.Sprintf("type %q is not one of PHOTO, VIDEO, AUDIO", input.Type))
		}
	}
	if input.Filename != "" {
		operator := "contains"
		params.FilenameVal = &input.Filename
		params.FilenameOperator = &operator
	}
	if input.Raw != nil {
		params.IsRaw = input.Raw
	}
	if input.Rating != nil {
		if *input.Rating < 0 || *input.Rating > 5 {
			return nil, ref.InvalidArgument(fmt.Sprintf("rating %d is out of range 0-5", *input.Rating))
		}
		rating := int32(*input.Rating)
		params.Rating = &rating
	}
	if input.Liked != nil {
		params.Liked = input.Liked
	}
	if input.Place != "" {
		params.Place = &input.Place
	}
	if input.Camera != "" {
		params.CameraModel = &input.Camera
	}
	if input.Lens != "" {
		params.LensModel = &input.Lens
	}
	if input.AlbumID != nil {
		if *input.AlbumID <= 0 {
			return nil, ref.InvalidArgument(fmt.Sprintf("album_id %d is not positive", *input.AlbumID))
		}
		albumID := int32(*input.AlbumID)
		params.AlbumID = &albumID
	}
	if len(input.TagNames) > 0 {
		params.TagNames = input.TagNames
	}
	return &params, nil
}

// filterHint derives the ref id mnemonic from the most distinctive condition.
func filterHint(input *AssetFilterInput) string {
	switch {
	case input.Place != "":
		return input.Place
	case input.Camera != "":
		return input.Camera
	case input.Lens != "":
		return input.Lens
	case input.AlbumID != nil:
		return fmt.Sprintf("album%d", *input.AlbumID)
	case input.Filename != "":
		return input.Filename
	case input.DateFrom != "" && len(input.DateFrom) >= 4:
		return input.DateFrom[:4]
	case input.Type != "":
		return strings.ToLower(input.Type)
	case input.Liked != nil && *input.Liked:
		return "liked"
	case len(input.TagNames) > 0:
		return input.TagNames[0]
	default:
		return "filter"
	}
}

func filterPlanParams(input *AssetFilterInput) map[string]string {
	params := map[string]string{}
	if input.DateFrom != "" {
		params["date_from"] = input.DateFrom
	}
	if input.DateTo != "" {
		params["date_to"] = input.DateTo
	}
	if input.Type != "" {
		params["type"] = input.Type
	}
	if input.Filename != "" {
		params["filename"] = input.Filename
	}
	if input.Raw != nil {
		params["raw"] = fmt.Sprintf("%t", *input.Raw)
	}
	if input.Rating != nil {
		params["rating"] = fmt.Sprintf("%d", *input.Rating)
	}
	if input.Liked != nil {
		params["liked"] = fmt.Sprintf("%t", *input.Liked)
	}
	if input.Place != "" {
		params["place"] = input.Place
	}
	if input.Camera != "" {
		params["camera"] = input.Camera
	}
	if input.Lens != "" {
		params["lens"] = input.Lens
	}
	if input.AlbumID != nil {
		params["album_id"] = fmt.Sprintf("%d", *input.AlbumID)
	}
	if len(input.TagNames) > 0 {
		params["tag_names"] = strings.Join(input.TagNames, ",")
	}
	return params
}

func filterSummary(input *AssetFilterInput, count int, truncated bool) string {
	params := filterPlanParams(input)
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	conditions := make([]string, 0, len(keys))
	for _, key := range keys {
		conditions = append(conditions, key+"="+params[key])
	}
	clause := strings.Join(conditions, ", ")
	if clause == "" {
		clause = "no conditions"
	}
	summary := fmt.Sprintf("filter(%s) → %d assets", clause, count)
	if count == 0 {
		summary += " (empty set)"
	}
	if truncated {
		summary += fmt.Sprintf(" (truncated at %d)", ref.MaxSnapshotSize)
	}
	return summary
}
