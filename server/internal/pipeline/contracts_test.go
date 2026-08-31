package pipeline

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func validEnvelopeCommand(t *testing.T, kind string) any {
	t.Helper()
	source, err := NewSourceFence(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "ingest_asset":
		return IngestCommand{CommitID: uuid.New(), ReceiptID: uuid.New(), Admission: AdmissionInteractive}
	case "asset.analyze":
		return AssetCommand{AssetID: uuid.New(), Fence: source, Stage: StageAnalyze, DesiredVersion: 1, PipelineVersion: AssetPipelineVersion, Admission: AdmissionBackground}
	case "asset.derivatives":
		return AssetCommand{AssetID: uuid.New(), Fence: source, Stage: StageDerivatives, DesiredVersion: 2, PipelineVersion: AssetPipelineVersion, Admission: AdmissionBackground}
	case "asset.transcode":
		return AssetCommand{AssetID: uuid.New(), Fence: source, Stage: StageTranscode, DesiredVersion: 3, PipelineVersion: AssetPipelineVersion, Admission: AdmissionMaintenance}
	case "asset.enrich":
		return AssetCommand{AssetID: uuid.New(), Fence: source, Stage: StageEnrich, DesiredVersion: 4, PipelineVersion: AssetPipelineVersion, Admission: AdmissionBackground}
	case "repository.scan":
		return RepositoryCommand{RepositoryID: uuid.New(), RequestedEpoch: 5, DesiredVersion: 5, Admission: AdmissionBackground}
	case "projection.event":
		return ProjectionCommand{Kind: "event", Scope: "7", SourceRevision: 6, ProjectionVersion: 2, Admission: AdmissionBackground}
	case "projection.location":
		return ProjectionCommand{Kind: "location", Scope: uuid.NewString() + ":9", SourceRevision: 8, ProjectionVersion: 8, Admission: AdmissionBackground}
	case "projection.ocr":
		return ProjectionCommand{Kind: "ocr", Scope: "all", SourceRevision: 9, ProjectionVersion: 3, Admission: AdmissionBackground}
	case "projection.asset_reindex":
		return ReindexCommand{ReceiptID: uuid.New(), Tasks: []string{"semantic"}, Limit: 10, RequestedRevision: 1, Admission: AdmissionMaintenance}
	case "backup_catalog":
		return BackupCommand{RequestID: uuid.New(), Admission: AdmissionMaintenance}
	default:
		t.Fatalf("unknown test kind %q", kind)
		return nil
	}
}

func TestNewEnvelopeAcceptsClosedTypedCommands(t *testing.T) {
	kinds := []string{
		"ingest_asset", "asset.analyze", "asset.derivatives", "asset.transcode", "asset.enrich",
		"repository.scan", "projection.event", "projection.location", "projection.ocr",
		"projection.asset_reindex", "backup_catalog",
	}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			command := validEnvelopeCommand(t, kind)
			envelope, err := NewEnvelope(kind, uuid.New(), uuid.New(), command, time.Now())
			if err != nil {
				t.Fatalf("NewEnvelope error: %v", err)
			}
			if err := validateEncodedCommand(kind, envelope.Payload); err != nil {
				t.Fatalf("encoded command rejected: %v", err)
			}
		})
	}
}

func TestNewEnvelopeRejectsStageKindAndAdmissionMismatches(t *testing.T) {
	source, err := NewSourceFence(uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, kind string
		command    any
	}{
		{
			name: "stage kind mismatch", kind: "asset.analyze",
			command: AssetCommand{AssetID: uuid.New(), Fence: source, Stage: StageEnrich, DesiredVersion: 1, PipelineVersion: AssetPipelineVersion, Admission: AdmissionBackground},
		},
		{
			name: "invalid admission", kind: "backup_catalog",
			command: BackupCommand{RequestID: uuid.New(), Admission: AdmissionClass("urgent")},
		},
		{
			name: "unknown kind", kind: "asset.unknown",
			command: AssetCommand{AssetID: uuid.New(), Fence: source, Stage: StageAnalyze, DesiredVersion: 1, PipelineVersion: AssetPipelineVersion, Admission: AdmissionBackground},
		},
		{
			name: "invalid location scope", kind: "projection.location",
			command: ProjectionCommand{Kind: "location", Scope: "not-a-scope", SourceRevision: 1, ProjectionVersion: 1, Admission: AdmissionBackground},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewEnvelope(test.kind, uuid.New(), uuid.New(), test.command, time.Now()); err == nil {
				t.Fatal("NewEnvelope accepted malformed command")
			}
		})
	}
}

func TestPublishEnvelopeTxRejectsIdentityDrift(t *testing.T) {
	database, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "pipeline.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec(`CREATE TABLE domain_outbox (
		outbox_id TEXT PRIMARY KEY,
		envelope_version INTEGER NOT NULL,
		command_kind TEXT NOT NULL,
		subject_key TEXT NOT NULL,
		desired_version INTEGER NOT NULL,
		envelope TEXT NOT NULL,
		available_at INTEGER NOT NULL,
		delivered_at INTEGER,
		delivery_attempts INTEGER NOT NULL,
		last_error TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		UNIQUE(command_kind, subject_key, desired_version)
	)`); err != nil {
		t.Fatal(err)
	}
	command := validEnvelopeCommand(t, "backup_catalog").(BackupCommand)
	envelope, err := NewEnvelope("backup_catalog", uuid.New(), command.RequestID, command, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	tx, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := PublishEnvelopeTx(context.Background(), tx, uuid.NewString(), 1, envelope); err == nil || !strings.Contains(err.Error(), "does not match command") {
		t.Fatalf("identity drift error = %v", err)
	}
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM domain_outbox`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("identity drift inserted %d rows", count)
	}
}

func TestRequestEventProjectionForceRearmsSameSource(t *testing.T) {
	database, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "projection.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec(`
		CREATE TABLE event_projection_pipeline_state (
			owner_id INTEGER PRIMARY KEY,
			source_revision INTEGER NOT NULL,
			projection_version INTEGER NOT NULL,
			applied_revision INTEGER NOT NULL,
			cursor TEXT,
			terminal_error TEXT,
			updated_at INTEGER NOT NULL
		);
		CREATE TABLE domain_outbox (
			outbox_id TEXT PRIMARY KEY,
			envelope_version INTEGER NOT NULL,
			command_kind TEXT NOT NULL,
			subject_key TEXT NOT NULL,
			desired_version INTEGER NOT NULL,
			envelope TEXT NOT NULL,
			available_at INTEGER NOT NULL,
			delivered_at INTEGER,
			delivery_attempts INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			UNIQUE(command_kind, subject_key, desired_version)
		)`); err != nil {
		t.Fatal(err)
	}

	first, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := RequestEventProjectionTx(context.Background(), first, 7, 4, false, uuid.New()); err != nil {
		_ = first.Rollback()
		t.Fatal(err)
	}
	if _, err := first.Exec(`UPDATE event_projection_pipeline_state SET applied_revision=4,cursor='resume-me' WHERE owner_id=7`); err != nil {
		_ = first.Rollback()
		t.Fatal(err)
	}
	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}

	second, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Rollback()
	if err := RequestEventProjectionTx(context.Background(), second, 7, 4, true, uuid.New()); err != nil {
		t.Fatal(err)
	}
	var source, version, applied uint64
	var cursor sql.NullString
	if err := second.QueryRow(`SELECT source_revision,projection_version,applied_revision,cursor FROM event_projection_pipeline_state WHERE owner_id=7`).Scan(&source, &version, &applied, &cursor); err != nil {
		t.Fatal(err)
	}
	if source != 4 || version != 2 || applied != 0 || cursor.Valid {
		t.Fatalf("forced projection state = source %d version %d applied %d cursor %#v", source, version, applied, cursor)
	}
}

func TestValidateEncodedCommandRejectsMalformedJSON(t *testing.T) {
	if err := validateEncodedCommand("backup_catalog", json.RawMessage(`{"request_id": "not-a-uuid"}`)); err == nil {
		t.Fatal("malformed encoded command was accepted")
	}
	if err := validateEncodedCommand("not-supported", []byte(`{}`)); err == nil {
		t.Fatal("unknown encoded command was accepted")
	}
	if err := validateEncodedCommand("backup_catalog", []byte(`{}`)); err == nil {
		t.Fatal("empty encoded command unexpectedly validated")
	}
}

func TestAssetDAGPublishesEnrichOnlyAfterPhotoDerivativesAreApplied(t *testing.T) {
	database, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "asset-dag.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	for _, statement := range []string{
		`CREATE TABLE assets(asset_id TEXT PRIMARY KEY,type TEXT,status TEXT,updated_at INTEGER)`,
		`CREATE TABLE catalog_operation_receipts(receipt_id TEXT PRIMARY KEY,kind TEXT,subject_id TEXT,desired_version INTEGER,applied_version INTEGER,state TEXT,terminal_error TEXT,created_at INTEGER,updated_at INTEGER)`,
		`CREATE TABLE asset_pipeline_state(asset_id TEXT,source_content_id TEXT,stage TEXT,pipeline_version TEXT,desired_version INTEGER,applied_version INTEGER,terminal_error TEXT,updated_at INTEGER,PRIMARY KEY(asset_id,stage))`,
		`CREATE TABLE asset_pipeline_receipt_stages(receipt_id TEXT,asset_id TEXT,stage TEXT,desired_version INTEGER,PRIMARY KEY(receipt_id,asset_id,stage))`,
		`CREATE TABLE domain_outbox(outbox_id TEXT PRIMARY KEY,envelope_version INTEGER,command_kind TEXT,subject_key TEXT,desired_version INTEGER,envelope TEXT,available_at INTEGER,delivered_at INTEGER,delivery_attempts INTEGER,last_error TEXT,created_at INTEGER,updated_at INTEGER,UNIQUE(command_kind,subject_key,desired_version))`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	assetID, contentID := uuid.New(), uuid.New()
	if _, err := database.Exec(`INSERT INTO assets VALUES(?, 'PHOTO', '{"state":"completed"}', 1)`, assetID.String()); err != nil {
		t.Fatal(err)
	}
	tx, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := RequestAssetStagesTx(context.Background(), tx, assetID, contentID, []Stage{StageAnalyze, StageDerivatives, StageEnrich}, AssetPipelineVersion, AdmissionInteractive, uuid.New()); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rows, err := database.Query(`SELECT command_kind FROM domain_outbox ORDER BY command_kind`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var kinds []string
	for rows.Next() {
		var kind string
		if err := rows.Scan(&kind); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, kind)
	}
	if got, want := strings.Join(kinds, ","), "asset.analyze,asset.derivatives"; got != want {
		t.Fatalf("initial outbox kinds = %s, want %s", got, want)
	}

	tx, err = database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`UPDATE asset_pipeline_state SET applied_version=desired_version WHERE asset_id=? AND stage='derivatives'`, assetID.String()); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := PublishReadyAssetStagesTx(context.Background(), tx, assetID, AdmissionBackground); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var enrich int
	if err := database.QueryRow(`SELECT count(*) FROM domain_outbox WHERE command_kind='asset.enrich'`).Scan(&enrich); err != nil || enrich != 1 {
		t.Fatalf("enrich outbox count = %d, err=%v", enrich, err)
	}
}
