package memory

import (
	"fmt"
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
	sections := make([]string, 0, 4)

	whatParts := make([]string, 0, 3)
	if strings.TrimSpace(e.Scenario) != "" {
		whatParts = append(whatParts, "scenario="+e.Scenario)
	}
	if strings.TrimSpace(e.Intent) != "" {
		whatParts = append(whatParts, "intent="+e.Intent)
	}
	if strings.TrimSpace(e.Summary) != "" {
		whatParts = append(whatParts, "summary="+e.Summary)
	}
	if len(whatParts) > 0 {
		sections = append(sections, "what:\n"+strings.Join(whatParts, "\n"))
	}

	if strings.TrimSpace(e.Goal) != "" {
		sections = append(sections, "goal:\n"+e.Goal)
	}

	entityParts := make([]string, 0, len(e.Entities))
	for _, entity := range e.Entities {
		name := strings.TrimSpace(entity.Name)
		if name == "" {
			continue
		}
		entityType := strings.TrimSpace(entity.Type)
		if entityType != "" {
			entityParts = append(entityParts, fmt.Sprintf("%s=%s", entityType, name))
			continue
		}
		entityParts = append(entityParts, name)
	}
	if len(entityParts) > 0 {
		sections = append(sections, "task_content:\n"+strings.Join(entityParts, "\n"))
	}

	toolNames := make([]string, 0, len(e.ToolTrace))
	for _, step := range e.ToolTrace {
		if strings.TrimSpace(step.Tool.Name) != "" {
			toolNames = append(toolNames, step.Tool.Name)
			continue
		}
		if strings.TrimSpace(step.Operation) != "" {
			toolNames = append(toolNames, step.Operation)
		}
	}
	if len(toolNames) > 0 {
		sections = append(sections, "procedure:\n"+strings.Join(toolNames, " -> "))
	}

	return strings.Join(sections, "\n\n")
}
