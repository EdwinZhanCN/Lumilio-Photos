// Package pipeline defines catalog-owned asynchronous work contracts.
// It deliberately has no dependency on River, SQL, or queue names.
package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Stage string

const (
	StageIngest      Stage = "ingest"
	StageAnalyze     Stage = "analyze"
	StageDerivatives Stage = "derivatives"
	StageTranscode   Stage = "transcode"
	StageEnrich      Stage = "enrich"
	StageRepository  Stage = "repository_scan"
	StageProjection  Stage = "projection_rebuild"
	StageBackup      Stage = "catalog_backup"
)

type AdmissionClass string

const (
	AdmissionInteractive AdmissionClass = "interactive"
	AdmissionBackground  AdmissionClass = "background"
	AdmissionMaintenance AdmissionClass = "maintenance"
)

func (a AdmissionClass) Valid() bool {
	switch a {
	case AdmissionInteractive, AdmissionBackground, AdmissionMaintenance:
		return true
	default:
		return false
	}
}

// SourceFence names the immutable source generation against which a result was
// computed. Asset fences contain content IDs; other domains use their typed
// epoch/revision fields instead of coercing them into this type.
type SourceFence uuid.UUID

func NewSourceFence(id uuid.UUID) (SourceFence, error) {
	if id == uuid.Nil {
		return SourceFence{}, errors.New("source fence must not be nil")
	}
	return SourceFence(id), nil
}

func (f SourceFence) UUID() uuid.UUID              { return uuid.UUID(f) }
func (f SourceFence) String() string               { return uuid.UUID(f).String() }
func (f SourceFence) MarshalJSON() ([]byte, error) { return json.Marshal(f.String()) }
func (f *SourceFence) UnmarshalJSON(data []byte) error {
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return err
	}
	id, err := uuid.Parse(encoded)
	if err != nil {
		return err
	}
	validated, err := NewSourceFence(id)
	if err != nil {
		return err
	}
	*f = validated
	return nil
}

type AssetCommand struct {
	AssetID         uuid.UUID      `json:"asset_id"`
	Fence           SourceFence    `json:"source_fence"`
	Stage           Stage          `json:"stage"`
	DesiredVersion  uint64         `json:"desired_version"`
	PipelineVersion string         `json:"pipeline_version"`
	Admission       AdmissionClass `json:"admission_class"`
}

type IngestCommand struct {
	CommitID  uuid.UUID      `json:"commit_id"`
	ReceiptID uuid.UUID      `json:"receipt_id"`
	Admission AdmissionClass `json:"admission_class"`
}

type RepositoryCommand struct {
	RepositoryID   uuid.UUID      `json:"repository_id"`
	RequestedEpoch uint64         `json:"requested_epoch"`
	Frontier       string         `json:"frontier,omitempty"`
	DesiredVersion uint64         `json:"desired_version"`
	Admission      AdmissionClass `json:"admission_class"`
}

type ProjectionCommand struct {
	Kind              string         `json:"kind"`
	Scope             string         `json:"scope"`
	SourceRevision    uint64         `json:"source_revision"`
	ProjectionVersion uint64         `json:"projection_version"`
	Cursor            string         `json:"cursor,omitempty"`
	Admission         AdmissionClass `json:"admission_class"`
}

type BackupCommand struct {
	RequestID uuid.UUID      `json:"request_id"`
	Force     bool           `json:"force"`
	Admission AdmissionClass `json:"admission_class"`
}

type ReindexCommand struct {
	ReceiptID         uuid.UUID      `json:"receipt_id"`
	RepositoryID      *uuid.UUID     `json:"repository_id,omitempty"`
	Tasks             []string       `json:"tasks"`
	Limit             int            `json:"limit"`
	Cursor            string         `json:"cursor,omitempty"`
	MissingOnly       bool           `json:"missing_only"`
	ResetSemantic     bool           `json:"reset_semantic"`
	RequestedRevision uint64         `json:"requested_revision"`
	Admission         AdmissionClass `json:"admission_class"`
}

// Envelope is the durable domain outbox value. Payload is a typed domain
// command encoded by NewEnvelope; it can never contain River insert options.
type Envelope struct {
	Version       uint16          `json:"version"`
	Kind          string          `json:"kind"`
	TraceID       uuid.UUID       `json:"trace_id"`
	CorrelationID uuid.UUID       `json:"correlation_id"`
	CreatedAt     time.Time       `json:"created_at"`
	Payload       json.RawMessage `json:"payload"`
}

const EnvelopeVersion uint16 = 1
const AssetPipelineVersion = "asset-v1"

func NewEnvelope(kind string, traceID, correlationID uuid.UUID, command any, now time.Time) (Envelope, error) {
	if kind == "" || traceID == uuid.Nil || correlationID == uuid.Nil {
		return Envelope{}, errors.New("outbox envelope requires kind, trace ID, and correlation ID")
	}
	if err := validateCommand(kind, command); err != nil {
		return Envelope{}, err
	}
	payload, err := json.Marshal(command)
	if err != nil {
		return Envelope{}, fmt.Errorf("encode domain command: %w", err)
	}
	return Envelope{Version: EnvelopeVersion, Kind: kind, TraceID: traceID, CorrelationID: correlationID, CreatedAt: now.UTC(), Payload: payload}, nil
}

func validateCommand(kind string, command any) error {
	switch kind {
	case "ingest_asset":
		value, ok := commandValue[IngestCommand](command)
		if !ok || value.CommitID == uuid.Nil || value.ReceiptID == uuid.Nil || !value.Admission.Valid() {
			return errors.New("invalid ingest command")
		}
	case "asset.analyze", "asset.derivatives", "asset.transcode", "asset.enrich":
		value, ok := commandValue[AssetCommand](command)
		if !ok || value.AssetID == uuid.Nil || value.Fence.UUID() == uuid.Nil || value.DesiredVersion == 0 || strings.TrimSpace(value.PipelineVersion) == "" || !value.Admission.Valid() {
			return errors.New("invalid asset command")
		}
		expectedKind := map[Stage]string{StageAnalyze: "asset.analyze", StageDerivatives: "asset.derivatives", StageTranscode: "asset.transcode", StageEnrich: "asset.enrich"}[value.Stage]
		if expectedKind != kind {
			return fmt.Errorf("asset command stage %q does not match envelope kind %q", value.Stage, kind)
		}
	case "repository.scan":
		value, ok := commandValue[RepositoryCommand](command)
		if !ok || value.RepositoryID == uuid.Nil || value.RequestedEpoch == 0 || value.RequestedEpoch != value.DesiredVersion || !value.Admission.Valid() {
			return errors.New("invalid repository command")
		}
	case "projection.event", "projection.location", "projection.ocr":
		value, ok := commandValue[ProjectionCommand](command)
		if !ok || value.Scope == "" || value.SourceRevision == 0 || value.ProjectionVersion == 0 || !value.Admission.Valid() {
			return errors.New("invalid projection command")
		}
		expectedKind := "projection.location"
		switch value.Kind {
		case "event":
			expectedKind = "projection.event"
		case "ocr":
			expectedKind = "projection.ocr"
		case "location", "location_resolution":
		default:
			return fmt.Errorf("unsupported projection kind %q", value.Kind)
		}
		if expectedKind != kind {
			return fmt.Errorf("projection command kind %q does not match envelope kind %q", value.Kind, kind)
		}
		if value.Kind == "event" {
			owner, err := strconv.ParseInt(value.Scope, 10, 32)
			if err != nil || owner <= 0 {
				return errors.New("event projection scope is invalid")
			}
		}
		if value.Kind == "location" {
			parts := strings.SplitN(value.Scope, ":", 2)
			if len(parts) != 2 {
				return errors.New("location projection scope is invalid")
			}
			repositoryID, err := uuid.Parse(parts[0])
			if err != nil || repositoryID == uuid.Nil {
				return errors.New("location projection repository is invalid")
			}
			owner, err := strconv.ParseInt(parts[1], 10, 32)
			if err != nil || owner <= 0 {
				return errors.New("location projection owner is invalid")
			}
			if value.SourceRevision != value.ProjectionVersion {
				return errors.New("location projection revision and version must match")
			}
		}
		if value.Kind == "location_resolution" || value.Kind == "ocr" {
			if value.Scope != "all" {
				return fmt.Errorf("%s projection scope must be all", value.Kind)
			}
		}
	case "projection.asset_reindex":
		value, ok := commandValue[ReindexCommand](command)
		if !ok || value.ReceiptID == uuid.Nil || value.RequestedRevision == 0 || value.Limit < 1 || value.Limit > 500 || len(value.Tasks) == 0 || !value.Admission.Valid() {
			return errors.New("invalid reindex command")
		}
		if value.RepositoryID != nil && *value.RepositoryID == uuid.Nil {
			return errors.New("invalid reindex repository")
		}
		for _, task := range value.Tasks {
			if strings.TrimSpace(task) == "" {
				return errors.New("reindex task name is empty")
			}
		}
	case "backup_catalog":
		value, ok := commandValue[BackupCommand](command)
		if !ok || value.RequestID == uuid.Nil || !value.Admission.Valid() {
			return errors.New("invalid backup command")
		}
	default:
		return fmt.Errorf("unsupported domain command kind %q", kind)
	}
	return nil
}

func commandValue[T any](command any) (T, bool) {
	switch value := command.(type) {
	case T:
		return value, true
	case *T:
		if value != nil {
			return *value, true
		}
	}
	var zero T
	return zero, false
}

func validateEncodedCommand(kind string, payload []byte) error {
	var command any
	switch kind {
	case "ingest_asset":
		command = &IngestCommand{}
	case "asset.analyze", "asset.derivatives", "asset.transcode", "asset.enrich":
		command = &AssetCommand{}
	case "repository.scan":
		command = &RepositoryCommand{}
	case "projection.event", "projection.location", "projection.ocr":
		command = &ProjectionCommand{}
	case "projection.asset_reindex":
		command = &ReindexCommand{}
	case "backup_catalog":
		command = &BackupCommand{}
	default:
		return fmt.Errorf("unsupported domain command kind %q", kind)
	}
	if err := json.Unmarshal(payload, command); err != nil {
		return fmt.Errorf("decode %s command: %w", kind, err)
	}
	return validateCommand(kind, command)
}

type ReceiptState string

const (
	ReceiptPending   ReceiptState = "pending"
	ReceiptCompleted ReceiptState = "completed"
	ReceiptFailed    ReceiptState = "failed"
)

type Receipt struct {
	ID             uuid.UUID    `json:"receipt_id"`
	Kind           string       `json:"kind"`
	State          ReceiptState `json:"state"`
	DesiredVersion uint64       `json:"desired_version"`
	AppliedVersion uint64       `json:"applied_version"`
	TerminalError  string       `json:"terminal_error,omitempty"`
}
