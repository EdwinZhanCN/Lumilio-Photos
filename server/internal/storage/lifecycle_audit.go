package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
)

const (
	AuditResultSucceeded = "succeeded"
	AuditResultFailed    = "failed"
	AuditResultRejected  = "rejected"
	AuditResultRecovered = "recovered"
)

// LifecycleAuditInput is the complete security and decision context for one
// lifecycle outcome. Paths are intentionally retained in the local catalog;
// SupportBundle performs the redaction boundary on export.
type LifecycleAuditInput struct {
	Actor            string
	ActorUserID      *int32
	HostInstanceID   string
	RequestID        string
	OperationID      string
	Action           string
	TargetType       string
	TargetID         string
	Source           string
	ConfirmationType string
	OldPath          string
	NewPath          string
	Result           string
	FailureStage     string
	Details          map[string]any
}

type LifecycleAuditEvent struct {
	EventID          string
	OccurredAt       time.Time
	Actor            string
	ActorUserID      *int32
	HostInstanceID   string
	RequestID        string
	OperationID      string
	Action           string
	TargetType       string
	TargetID         string
	Source           string
	ConfirmationType string
	OldPath          string
	NewPath          string
	Result           string
	FailureStage     string
	Details          json.RawMessage
}

type LifecycleAuditFilter struct {
	TargetType string
	TargetID   string
	Limit      int64
	Offset     int64
}

func (rm *DefaultRepositoryManager) RecordLifecycleAudit(ctx context.Context, input LifecycleAuditInput) (LifecycleAuditEvent, error) {
	if rm == nil || rm.queries == nil {
		return LifecycleAuditEvent{}, fmt.Errorf("lifecycle audit catalog is unavailable")
	}
	return recordLifecycleAuditWithQueries(ctx, rm.queries, input)
}

func recordLifecycleAuditWithQueries(ctx context.Context, queries *repo.Queries, input LifecycleAuditInput) (LifecycleAuditEvent, error) {
	normalizeLifecycleAuditInput(&input)
	details, err := json.Marshal(input.Details)
	if err != nil {
		return LifecycleAuditEvent{}, fmt.Errorf("encode lifecycle audit details: %w", err)
	}
	var actorUserID *int64
	if input.ActorUserID != nil {
		value := int64(*input.ActorUserID)
		actorUserID = &value
	}
	row, err := queries.InsertLifecycleAuditEvent(ctx, repo.InsertLifecycleAuditEventParams{
		EventID: uuid.NewString(), OccurredAt: time.Now().UTC().UnixMilli(),
		Actor: input.Actor, ActorUserID: actorUserID, HostInstanceID: input.HostInstanceID,
		RequestID: input.RequestID, OperationID: optionalAuditString(input.OperationID),
		Action: input.Action, TargetType: input.TargetType, TargetID: optionalAuditString(input.TargetID),
		Source: input.Source, ConfirmationType: input.ConfirmationType,
		OldPath: optionalAuditString(input.OldPath), NewPath: optionalAuditString(input.NewPath),
		Result: input.Result, FailureStage: optionalAuditString(input.FailureStage), Details: string(details),
	})
	if err != nil {
		return LifecycleAuditEvent{}, fmt.Errorf("persist lifecycle audit event: %w", err)
	}
	return lifecycleAuditEventFromRow(row), nil
}

func (rm *DefaultRepositoryManager) ListLifecycleAudit(ctx context.Context, filter LifecycleAuditFilter) ([]LifecycleAuditEvent, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	var rows []repo.LifecycleAuditEvent
	var err error
	if strings.TrimSpace(filter.TargetType) != "" || strings.TrimSpace(filter.TargetID) != "" {
		if strings.TrimSpace(filter.TargetType) == "" || strings.TrimSpace(filter.TargetID) == "" {
			return nil, fmt.Errorf("target_type and target_id must be provided together")
		}
		rows, err = rm.queries.ListLifecycleAuditEventsForTarget(ctx, repo.ListLifecycleAuditEventsForTargetParams{
			TargetType: strings.TrimSpace(filter.TargetType), TargetID: optionalAuditString(filter.TargetID),
			Limit: limit, Offset: filter.Offset,
		})
	} else {
		rows, err = rm.queries.ListLifecycleAuditEvents(ctx, repo.ListLifecycleAuditEventsParams{Limit: limit, Offset: filter.Offset})
	}
	if err != nil {
		return nil, fmt.Errorf("list lifecycle audit events: %w", err)
	}
	items := make([]LifecycleAuditEvent, 0, len(rows))
	for _, row := range rows {
		items = append(items, lifecycleAuditEventFromRow(row))
	}
	return items, nil
}

func normalizeLifecycleAuditInput(input *LifecycleAuditInput) {
	input.Actor = strings.TrimSpace(input.Actor)
	if input.Actor == "" {
		input.Actor = "server"
	}
	input.Action = strings.TrimSpace(input.Action)
	input.TargetType = strings.TrimSpace(input.TargetType)
	if input.TargetType == "" {
		input.TargetType = "runtime_config"
	}
	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		input.Source = auditSourceForActor(input.Actor)
	}
	input.ConfirmationType = strings.TrimSpace(input.ConfirmationType)
	if input.ConfirmationType == "" {
		input.ConfirmationType = "none"
	}
	input.Result = strings.TrimSpace(input.Result)
	if input.Result == "" {
		input.Result = AuditResultSucceeded
	}
	if input.Details == nil {
		input.Details = map[string]any{}
	}
}

func auditSourceForActor(actor string) string {
	switch {
	case strings.HasPrefix(actor, "web:"):
		return "web"
	case strings.HasPrefix(actor, "desktop_host"):
		return "desktop_host"
	case strings.Contains(actor, "recovery"):
		return "recovery"
	case strings.HasPrefix(actor, "test"):
		return "test"
	default:
		return "server"
	}
}

func optionalAuditString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func lifecycleAuditEventFromRow(row repo.LifecycleAuditEvent) LifecycleAuditEvent {
	var userID *int32
	if row.ActorUserID != nil {
		value := int32(*row.ActorUserID)
		userID = &value
	}
	return LifecycleAuditEvent{
		EventID: row.EventID, OccurredAt: time.UnixMilli(row.OccurredAt).UTC(), Actor: row.Actor,
		ActorUserID: userID, HostInstanceID: row.HostInstanceID, RequestID: row.RequestID,
		OperationID: auditStringValue(row.OperationID), Action: row.Action, TargetType: row.TargetType,
		TargetID: auditStringValue(row.TargetID), Source: row.Source, ConfirmationType: row.ConfirmationType,
		OldPath: auditStringValue(row.OldPath), NewPath: auditStringValue(row.NewPath), Result: row.Result,
		FailureStage: auditStringValue(row.FailureStage), Details: json.RawMessage(row.Details),
	}
}

func auditStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
