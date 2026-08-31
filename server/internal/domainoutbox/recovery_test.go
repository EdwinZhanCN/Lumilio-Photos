package domainoutbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"server/config"
	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/storage/repocfg"
)

func TestReconcilerRetriesTransientCatalogErrorsUntilCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// A closed *sql.DB is a failed catalog capability. The reconciler must keep
	// its lifecycle alive for a later recovery pass instead of converting one
	// transient catalog error into a permanently stopped goroutine.
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	if catalog.ReaderSQL != nil {
		_ = catalog.ReaderSQL.Close()
	}
	writer := catalog.Writer
	err = NewReconciler(writer, time.Millisecond).Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want context deadline exceeded after retrying", err)
	}
}

func TestBackupSchedulerRetriesTransientCatalogErrorsUntilCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.SQL.Close(); err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewBackupScheduler(catalog.Writer, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	err = scheduler.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want context deadline exceeded after retrying", err)
	}
}

type recordingAdapter struct{ entries []Entry }

func (a *recordingAdapter) InsertMany(_ context.Context, entries []Entry) error {
	a.entries = append(a.entries, entries...)
	return nil
}

func TestPendingReceiptIsRedeliveredAfterQueueDatabaseLoss(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close(context.Background())
	if err := catalog.MigrateCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	commitID, receiptID := uuid.New(), uuid.New()
	if err := catalog.Writer.Transact(ctx, catalogtx.OperationAssetStagingCommit, nil, func(tx *sql.Tx) error { return pipeline.RequestIngestTx(ctx, tx, commitID, receiptID, uuid.New()) }); err != nil {
		t.Fatal(err)
	}
	adapter := &recordingAdapter{}
	dispatcher, err := NewDispatcher(catalog.Reader, catalog.Writer, adapter, 10, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := dispatcher.DeliverOnce(ctx); err != nil || count != 1 {
		t.Fatalf("first delivery count=%d err=%v", count, err)
	}
	adapter.entries = nil // deleting/recreating river.db loses the delivered macro job
	reconciler := NewReconciler(catalog.Writer, time.Second)
	if _, err := reconciler.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := dispatcher.DeliverOnce(ctx); err != nil || count != 1 {
		t.Fatalf("recovery delivery count=%d err=%v", count, err)
	}
	if len(adapter.entries) != 1 || adapter.entries[0].Envelope.Kind != "ingest_asset" {
		t.Fatalf("recovered entries=%+v", adapter.entries)
	}
}

func TestCoalescedRepositoryRunIsRedeliveredAtActiveEpochAfterQueueDatabaseLoss(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close(context.Background())
	if err := catalog.MigrateCatalog(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "repository-recovery-owner", Password: "unused", DisplayName: "Repository Recovery Owner",
		Role: "admin", WebauthnUserHandle: []byte("repository-recovery-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	rootID := uuid.New()
	if _, err := catalog.Queries.UpsertRepositoryRoot(ctx, repo.UpsertRepositoryRootParams{
		RootID: rootID, Name: "repository recovery root", Path: t.TempDir(),
		Kind: dbtypes.RepositoryRootKindExternal, Status: dbtypes.RepositoryRootStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	repositoryID := uuid.New()
	repositoryConfig := repocfg.NewRepositoryConfig("repository recovery")
	repositoryConfig.ID = repositoryID.String()
	if _, err := catalog.Queries.CreateRepository(ctx, repo.CreateRepositoryParams{
		RepoID: repositoryID, Name: "repository recovery", Path: t.TempDir(), Config: *repositoryConfig,
		Role: dbtypes.RepoRoleRegular, Reachability: dbtypes.RepositoryReachabilityActive,
		Activity: dbtypes.RepositoryActivityIdle, DefaultOwnerID: &owner.UserID,
		CreatedAt: now, UpdatedAt: now, RootID: rootID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.EnsureRepositoryObservationState(ctx, repo.EnsureRepositoryObservationStateParams{
		RepositoryID: repositoryID, AdapterKind: "periodic", VolumeKind: "unknown",
		PathCaseMode: "sensitive", PathNormalization: "none", CursorHealth: "unavailable",
		FullVerificationRequired: 1, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	var state repo.RepositoryObservationState
	for range 5 {
		state, err = catalog.Queries.RequestRepositoryObservationEpoch(ctx, repo.RequestRepositoryObservationEpochParams{
			RepositoryID: repositoryID, FullVerificationRequired: 1, UpdatedAt: now,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	runID := uuid.New()
	if _, err := catalog.Queries.CreateRepositoryScanRun(ctx, repo.CreateRepositoryScanRunParams{
		RunID: runID, RepositoryID: repositoryID, RequestedEpoch: 1, Mode: "periodic",
		ForceFullVerification: 1, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.SetActiveRepositoryObservationRun(ctx, repo.SetActiveRepositoryObservationRunParams{
		RepositoryID: repositoryID, ActiveRunID: uuid.NullUUID{UUID: runID, Valid: true}, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := NewReconciler(catalog.Writer, time.Second).ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	adapter := &recordingAdapter{}
	dispatcher, err := NewDispatcher(catalog.Reader, catalog.Writer, adapter, 10, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := dispatcher.DeliverOnce(ctx); err != nil || count != 1 {
		t.Fatalf("repository recovery delivery count=%d err=%v", count, err)
	}
	if len(adapter.entries) != 1 || adapter.entries[0].Envelope.Kind != "repository.scan" {
		t.Fatalf("recovered entries=%+v", adapter.entries)
	}
	var command pipeline.RepositoryCommand
	if err := json.Unmarshal(adapter.entries[0].Envelope.Payload, &command); err != nil {
		t.Fatal(err)
	}
	if state.DesiredEpoch != 5 || command.RepositoryID != repositoryID || command.RequestedEpoch != 1 || command.DesiredVersion != 1 {
		t.Fatalf("coalesced state=%+v recovered command=%+v", state, command)
	}
}

func TestPendingBackupRequestIsRedeliveredFromCatalogTruth(t *testing.T) {
	ctx := context.Background()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(directory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close(context.Background())
	if err := catalog.MigrateCatalog(ctx); err != nil {
		t.Fatal(err)
	}

	scheduler, err := NewBackupScheduler(catalog.Writer, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	receiptID, err := scheduler.Request(ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	adapter := &recordingAdapter{}
	dispatcher, err := NewDispatcher(catalog.Reader, catalog.Writer, adapter, 10, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := dispatcher.DeliverOnce(ctx); err != nil || count != 1 {
		t.Fatalf("first delivery count=%d err=%v", count, err)
	}
	adapter.entries = nil
	if _, err := NewReconciler(catalog.Writer, time.Second).ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := dispatcher.DeliverOnce(ctx); err != nil || count != 1 {
		t.Fatalf("recovery delivery count=%d err=%v", count, err)
	}
	if len(adapter.entries) != 1 || adapter.entries[0].Envelope.Kind != "backup_catalog" {
		t.Fatalf("recovered entries=%+v", adapter.entries)
	}
	var command pipeline.BackupCommand
	if err := json.Unmarshal(adapter.entries[0].Envelope.Payload, &command); err != nil {
		t.Fatal(err)
	}
	if command.RequestID != receiptID || !command.Force {
		t.Fatalf("recovered command=%+v", command)
	}
}
