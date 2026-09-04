// Package pipeline defines catalog-owned asynchronous work contracts.
// It deliberately has no dependency on River, SQL, or queue names.
package pipeline

import (
	"encoding/json"
	"errors"

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

const AssetPipelineVersion = "asset-v1"

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
