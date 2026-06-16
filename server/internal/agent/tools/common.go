package tools

import (
	"fmt"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/ref"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// RefToolOutput is the uniform output of every ref-producing tool (INV-3):
// either a receipt or a typed, recoverable error — never asset data (INV-1).
type RefToolOutput struct {
	Receipt *ref.ToolReceipt `json:"receipt,omitempty"`
	Error   *ref.Error       `json:"error,omitempty"`
}

func receiptOutput(r *ref.Ref, summary string) *RefToolOutput {
	return &RefToolOutput{
		Receipt: &ref.ToolReceipt{
			RefID:   r.ID,
			Count:   r.Count(),
			Summary: summary,
		},
	}
}

func errorOutput(e *ref.Error) *RefToolOutput {
	return &RefToolOutput{Error: e}
}

func newExecutionID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func sendRunning(deps *core.ToolDependencies, tool, execID, message string, params interface{}) {
	deps.Send(&core.SideChannelEvent{
		Type:      core.EventTypeToolExecution,
		Timestamp: time.Now().UnixMilli(),
		Tool:      core.ToolIdentity{Name: tool, ExecutionID: execID},
		Execution: core.ExecutionInfo{
			Status:     core.ExecutionStatusRunning,
			Message:    message,
			Parameters: params,
		},
	})
}

func sendSuccess(deps *core.ToolDependencies, tool, execID string, start time.Time, message string, data *core.DataPayload) {
	deps.Send(&core.SideChannelEvent{
		Type:      core.EventTypeToolExecution,
		Timestamp: time.Now().UnixMilli(),
		Tool:      core.ToolIdentity{Name: tool, ExecutionID: execID},
		Execution: core.ExecutionInfo{
			Status:   core.ExecutionStatusSuccess,
			Message:  message,
			Duration: time.Since(start).Milliseconds(),
		},
		Data: data,
	})
}

func sendError(deps *core.ToolDependencies, tool, execID string, start time.Time, e *ref.Error) {
	deps.Send(&core.SideChannelEvent{
		Type:      core.EventTypeToolExecution,
		Timestamp: time.Now().UnixMilli(),
		Tool:      core.ToolIdentity{Name: tool, ExecutionID: execID},
		Execution: core.ExecutionInfo{
			Status:   core.ExecutionStatusError,
			Message:  e.Message,
			Duration: time.Since(start).Milliseconds(),
			Error:    &core.ErrorInfo{Code: string(e.Code), Message: e.Message, Hint: e.Hint},
		},
	})
}

func toPgUUIDs(ids []uuid.UUID) []pgtype.UUID {
	out := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		out[i] = pgtype.UUID{Bytes: id, Valid: true}
	}
	return out
}

func fromPgUUIDs(ids []pgtype.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id.Valid {
			out = append(out, uuid.UUID(id.Bytes))
		}
	}
	return out
}

// RegisterAll registers the full toolset. Call once at bootstrap.
func RegisterAll() {
	// Producers
	RegisterFilterAssets()
	RegisterSearchSemantic()
	RegisterSearchText()
	RegisterSearchPeople()
	// Lookup
	RegisterLookupPeople()
	RegisterLookupAlbums()
	// Transformers
	RegisterCombine()
	RegisterRank()
	RegisterTop()
	RegisterSample()
	// Observers
	RegisterDescribe()
	RegisterPeek()
	RegisterInspect()
	// Terminals
	RegisterShow()
	RegisterBulkLike()
	RegisterCreateAlbum()
	RegisterAddToAlbum()
}
