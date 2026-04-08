package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"server/internal/agent/core"
)

type EpisodeStatus string

const (
	EpisodeStatusSucceeded EpisodeStatus = "succeeded"
	EpisodeStatusFailed    EpisodeStatus = "failed"
	EpisodeStatusRecovered EpisodeStatus = "recovered"
	EpisodeStatusAborted   EpisodeStatus = "aborted"
)

type WriteTrigger string

const (
	WriteTriggerGoalResolved   WriteTrigger = "goal_resolved"
	WriteTriggerErrorCaptured  WriteTrigger = "error_captured"
	WriteTriggerErrorRecovered WriteTrigger = "error_recovered"
	WriteTriggerManualBookmark WriteTrigger = "manual_bookmark"
)

type EntityRef struct {
	Type string `json:"type"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

type ContextBlock struct {
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	Text   string  `json:"text"`
	Weight float32 `json:"weight,omitempty"`
}

type ToolTraceStep struct {
	Index         int                    `json:"index"`
	Tool          core.ToolIdentity      `json:"tool"`
	Operation     string                 `json:"operation,omitempty"`
	Input         map[string]any         `json:"input,omitempty"`
	OutputSummary string                 `json:"output_summary,omitempty"`
	Status        core.ExecutionStatus   `json:"status"`
	Error         *core.ErrorInfo        `json:"error,omitempty"`
	FixSummary    string                 `json:"fix_summary,omitempty"`
	StartedAt     time.Time              `json:"started_at"`
	FinishedAt    time.Time              `json:"finished_at"`
	SideEvent     *core.SideChannelEvent `json:"side_event,omitempty"`
}

type Episode struct {
	ID            string            `json:"id"`
	ThreadID      string            `json:"thread_id"`
	UserID        string            `json:"user_id"`
	AgentName     string            `json:"agent_name"`
	Scenario      string            `json:"scenario"`
	Goal          string            `json:"goal"`
	Intent        string            `json:"intent"`
	Summary       string            `json:"summary"`
	RetrievalText string            `json:"retrieval_text"`
	Workspace     string            `json:"workspace,omitempty"`
	Route         string            `json:"route,omitempty"`
	Status        EpisodeStatus     `json:"status"`
	WriteTrigger  WriteTrigger      `json:"write_trigger"`
	StartedAt     time.Time         `json:"started_at"`
	EndedAt       time.Time         `json:"ended_at"`
	Tags          []string          `json:"tags,omitempty"`
	Entities      []EntityRef       `json:"entities,omitempty"`
	Refs          []string          `json:"refs,omitempty"`
	ToolTrace     []ToolTraceStep   `json:"tool_trace,omitempty"`
	ContextBlocks []ContextBlock    `json:"context_blocks,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type SearchRequest struct {
	Query        string
	UserID       string
	Goal         string
	Intent       string
	Entity       string
	Status       EpisodeStatus
	Tags         []string
	Limit        int
	StartedAfter *time.Time
	EndedBefore  *time.Time
}

type SearchHit struct {
	ID      string         `json:"id"`
	Score   float32        `json:"score"`
	Episode Episode        `json:"episode"`
	Payload map[string]any `json:"payload,omitempty"`
}

func (e Episode) BuildRetrievalText() string {
	parts := make([]string, 0, 8)

	if e.Goal != "" {
		parts = append(parts, "goal: "+e.Goal)
	}
	if e.Intent != "" {
		parts = append(parts, "intent: "+e.Intent)
	}
	if e.Scenario != "" {
		parts = append(parts, "scenario: "+e.Scenario)
	}
	if e.Summary != "" {
		parts = append(parts, "summary: "+e.Summary)
	}
	if len(e.Entities) > 0 {
		entityParts := make([]string, 0, len(e.Entities))
		for _, entity := range e.Entities {
			if entity.Type != "" {
				entityParts = append(entityParts, fmt.Sprintf("%s=%s", entity.Type, entity.Name))
				continue
			}
			entityParts = append(entityParts, entity.Name)
		}
		parts = append(parts, "entities: "+strings.Join(entityParts, ", "))
	}
	if len(e.Metadata) > 0 {
		keys := make([]string, 0, len(e.Metadata))
		for key, value := range e.Metadata {
			if strings.TrimSpace(key) == "" || key == "cluster_id" || strings.TrimSpace(value) == "" {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		slotParts := make([]string, 0, len(keys))
		for _, key := range keys {
			slotParts = append(slotParts, fmt.Sprintf("%s=%s", key, e.Metadata[key]))
		}
		if len(slotParts) > 0 {
			parts = append(parts, "slots: "+strings.Join(slotParts, ", "))
		}
	}
	if len(e.Tags) > 0 {
		parts = append(parts, "tags: "+strings.Join(e.Tags, ", "))
	}
	if len(e.ToolTrace) > 0 {
		toolNames := make([]string, 0, len(e.ToolTrace))
		for _, step := range e.ToolTrace {
			if step.Tool.Name != "" {
				toolNames = append(toolNames, step.Tool.Name)
			}
		}
		if len(toolNames) > 0 {
			parts = append(parts, "tools: "+strings.Join(toolNames, " -> "))
		}
	}
	parts = append(parts, "status: "+string(e.Status))

	return strings.Join(parts, "\n")
}
