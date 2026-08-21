package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"server/internal/agent/core"
	"server/internal/agent/ref"
	"server/internal/db/repo"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

const (
	maxReadOCRRefCount = 2
	maxOCRLineRunes    = 500
	maxOCRTotalRunes   = 6000
)

const (
	ocrStatusAvailable       = "available"
	ocrStatusNotAvailable    = "not_available"
	ocrStatusUnsupportedType = "unsupported_type"
)

// ReadOCRInput identifies a small ref whose stored OCR content should be read.
type ReadOCRInput struct {
	RefID string `json:"ref_id" jsonschema:"description=Ref to read stored OCR Text Recognition content from (max 2 assets)"`
}

// ReadOCRDocument is a bounded, sanitized OCR view for one ref member.
type ReadOCRDocument struct {
	Position    int      `json:"position"`
	Filename    string   `json:"filename"`
	Status      string   `json:"status"`
	RegionCount int64    `json:"region_count"`
	Lines       []string `json:"lines"`
	Truncated   bool     `json:"truncated"`
}

// ReadOCROutput preserves ref order without exposing internal asset IDs or
// model-specific OCR fields.
type ReadOCROutput struct {
	RefID     string            `json:"ref_id,omitempty"`
	Documents []ReadOCRDocument `json:"documents,omitempty"`
	Error     *ref.Error        `json:"error,omitempty"`
}

// RegisterReadOCR registers the bounded observer for authoritative stored OCR.
func RegisterReadOCR() {
	info := &schema.ToolInfo{
		Name: "read_ocr",
		Desc: "Read stored OCR Text Recognition content for refs with at most 2 assets. " +
			"Use search_text or another producer to narrow the ref first. Returns sanitized text in provider order; " +
			"it does not run OCR or expose confidence, geometry, model IDs, or asset IDs.",
	}

	core.GetRegistry().Register(info, func(ctx context.Context, deps *core.ToolDependencies) (tool.BaseTool, error) {
		return utils.InferTool(info.Name, info.Desc, func(ctx context.Context, input *ReadOCRInput) (*ReadOCROutput, error) {
			start := time.Now()
			execID := newExecutionID()

			r, refErr := deps.ResolveRef(ctx, input.RefID)
			if refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &ReadOCROutput{Error: refErr}, nil
			}
			if refErr = validateReadOCRRef(r); refErr != nil {
				sendError(deps, info.Name, execID, start, refErr)
				return &ReadOCROutput{Error: refErr}, nil
			}

			rows, err := deps.Library.ReadOCRDocuments(ctx, r.AssetIDs)
			if err != nil {
				refErr := ref.Internal("read OCR query")
				sendError(deps, info.Name, execID, start, refErr)
				return &ReadOCROutput{Error: refErr}, nil
			}
			documents := formatOCRDocuments(r.AssetIDs, rows)
			available := 0
			for _, document := range documents {
				if document.Status == ocrStatusAvailable {
					available++
				}
			}

			sendSuccess(deps, info.Name, execID, start,
				fmt.Sprintf("Read stored OCR for %d of %d assets", available, len(documents)),
				&core.DataPayload{RefID: r.ID, Count: r.Count()})
			return &ReadOCROutput{RefID: r.ID, Documents: documents}, nil
		})
	})
}

func validateReadOCRRef(r *ref.Ref) *ref.Error {
	if r.Count() == 0 {
		return ref.EmptySet(r.ID)
	}
	if r.Count() > maxReadOCRRefCount {
		return ref.InvalidArgument(fmt.Sprintf(
			"read_ocr supports at most %d assets; this ref has %d — narrow it with top or sample first",
			maxReadOCRRefCount,
			r.Count(),
		))
	}
	return nil
}

func formatOCRDocuments(ids []uuid.UUID, rows []repo.AgentReadOCRDocumentsRow) []ReadOCRDocument {
	byID := make(map[uuid.UUID][]repo.AgentReadOCRDocumentsRow, len(ids))
	for _, row := range rows {
		byID[row.AssetID] = append(byID[row.AssetID], row)
	}

	documents := make([]ReadOCRDocument, 0, len(ids))
	remaining := maxOCRTotalRunes
	for index, id := range ids {
		assetRows := byID[id]
		document := ReadOCRDocument{
			Position: index + 1,
			Status:   ocrStatusNotAvailable,
			Lines:    []string{},
		}
		if len(assetRows) == 0 {
			documents = append(documents, document)
			continue
		}

		first := assetRows[0]
		document.Filename = ref.SanitizeUserText(first.OriginalFilename, ref.MaxPeekFieldLen)
		document.RegionCount = first.RegionCount
		switch {
		case !strings.EqualFold(first.Type, "PHOTO"):
			document.Status = ocrStatusUnsupportedType
		case first.HasOcrResult == 0:
			document.Status = ocrStatusNotAvailable
		default:
			document.Status = ocrStatusAvailable
			for _, row := range assetRows {
				line := sanitizeOCRLine(row.TextContent)
				if line == "" {
					continue
				}
				lineRunes := utf8.RuneCountInString(line)
				if lineRunes > remaining {
					document.Truncated = true
					break
				}
				document.Lines = append(document.Lines, line)
				remaining -= lineRunes
			}
		}
		documents = append(documents, document)
	}
	return documents
}

func sanitizeOCRLine(value string) string {
	value = ref.SanitizeUserText(value, 0)
	runes := []rune(value)
	if len(runes) <= maxOCRLineRunes {
		return value
	}
	return string(runes[:maxOCRLineRunes-1]) + "…"
}
