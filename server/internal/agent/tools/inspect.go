package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/dbtypes"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

const maxInspectRefCount = 3

// InspectInput returns per-asset EXIF facets for a small ref.
type InspectInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref to inspect (max 3 assets)"`
}

// InspectOutput holds sanitized facet lines per asset.
type InspectOutput struct {
	RefID string     `json:"ref_id,omitempty"`
	Lines []string   `json:"lines,omitempty"`
	Error *ref.Error `json:"error,omitempty"`
}

// RegisterInspect registers the inspect observer for detailed EXIF facets.
func RegisterInspect() {
	info := &schema.ToolInfo{
		Name: "inspect",
		Desc: "Return per-asset camera EXIF facets (camera, lens, focal length, aperture, shutter, ISO, dimensions) " +
			"for refs with at most 3 assets. Use describe for aggregate views of larger sets.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *InspectInput) (*InspectOutput, error) {
			start := time.Now()
			execID := newExecutionID()

			r, refErr := deps.RefStore.Get(deps.Scope(), input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &InspectOutput{Error: refErr}, nil
			}
			if r.Count() == 0 {
				refErr := ref.EmptySet(r.ID)
				sendError(deps, info.Name, execID, start, refErr)
				return &InspectOutput{Error: refErr}, nil
			}
			if r.Count() > maxInspectRefCount {
				refErr := ref.InvalidArgument(fmt.Sprintf("inspect supports at most %d assets; this ref has %d — use describe instead", maxInspectRefCount, r.Count()))
				sendError(deps, info.Name, execID, start, refErr)
				return &InspectOutput{Error: refErr}, nil
			}

			rows, err := deps.Queries.AgentInspectAssets(ctx, toPgUUIDs(r.AssetIDs))
			if err != nil {
				refErr := ref.Internal("inspect query")
				sendError(deps, info.Name, execID, start, refErr)
				return &InspectOutput{Error: refErr}, nil
			}

			byID := make(map[uuid.UUID]string, len(rows))
			for _, row := range rows {
				byID[uuid.UUID(row.AssetID.Bytes)] = formatInspectLine(row.Type, []byte(row.SpecificMetadata))
			}

			lines := make([]string, 0, len(r.AssetIDs))
			for _, id := range r.AssetIDs {
				if line, ok := byID[id]; ok && line != "" {
					lines = append(lines, line)
				}
			}

			sendSuccess(deps, info.Name, execID, start,
				fmt.Sprintf("Inspected %d assets", len(lines)),
				&core.DataPayload{RefID: r.ID, Count: r.Count()})
			return &InspectOutput{RefID: r.ID, Lines: lines}, nil
		})
	})
}

func formatInspectLine(assetType string, metadata []byte) string {
	if len(metadata) == 0 {
		return strings.ToLower(assetType)
	}

	var parts []string
	parts = append(parts, strings.ToLower(assetType))

	switch strings.ToUpper(assetType) {
	case "PHOTO":
		var meta dbtypes.PhotoSpecificMetadata
		if err := json.Unmarshal(metadata, &meta); err != nil {
			break
		}
		if meta.CameraModel != "" {
			parts = append(parts, "camera="+ref.SanitizeUserText(meta.CameraModel, ref.MaxPeekFieldLen))
		}
		if meta.LensModel != "" {
			parts = append(parts, "lens="+ref.SanitizeUserText(meta.LensModel, ref.MaxPeekFieldLen))
		}
		if meta.FocalLength > 0 {
			parts = append(parts, fmt.Sprintf("focal=%.0fmm", meta.FocalLength))
		}
		if meta.FNumber > 0 {
			parts = append(parts, fmt.Sprintf("f/%g", meta.FNumber))
		}
		if meta.ExposureTime != "" {
			parts = append(parts, "shutter="+ref.SanitizeUserText(meta.ExposureTime, ref.MaxPeekFieldLen))
		}
		if meta.IsoSpeed > 0 {
			parts = append(parts, fmt.Sprintf("ISO %d", meta.IsoSpeed))
		}
		if meta.Dimensions != "" {
			parts = append(parts, "size="+ref.SanitizeUserText(meta.Dimensions, ref.MaxPeekFieldLen))
		}
	default:
		var raw map[string]any
		if err := json.Unmarshal(metadata, &raw); err == nil {
			if v, ok := raw["camera_model"].(string); ok && v != "" {
				parts = append(parts, "camera="+ref.SanitizeUserText(v, ref.MaxPeekFieldLen))
			}
		}
	}

	return strings.Join(parts, " · ")
}
