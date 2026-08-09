package cloud

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type blockingCredentialProvider struct{ importer CloudProvider }

func (blockingCredentialProvider) Descriptor() ProviderDescriptor {
	return ProviderDescriptor{ID: ProviderICloud, Status: ProviderStatusEnabled}
}

func (blockingCredentialProvider) Identity(map[string]string) (CredentialIdentity, error) {
	return CredentialIdentity{}, nil
}

func (blockingCredentialProvider) DefaultArtifactDir(uuid.UUID) string { return "" }

func (blockingCredentialProvider) Authenticate(context.Context, CredentialAuthInput) (CredentialAuthResult, error) {
	return CredentialAuthResult{}, nil
}

func (blockingCredentialProvider) VerifyChallenge(context.Context, CredentialChallengeInput) (CredentialAuthResult, error) {
	return CredentialAuthResult{}, nil
}

func (provider blockingCredentialProvider) NewImporter(context.Context, repo.CloudCredential) (CloudProvider, error) {
	return provider.importer, nil
}

type blockingCloudProvider struct {
	cancelObserved chan struct{}
	release        chan struct{}
	once           sync.Once
}

func (provider *blockingCloudProvider) List(ctx context.Context, _ uuid.UUID, _ *Cursor, _ map[string]string) (*Page, error) {
	<-ctx.Done()
	provider.once.Do(func() { close(provider.cancelObserved) })
	<-provider.release
	return nil, ctx.Err()
}

func (*blockingCloudProvider) Name() ProviderKind { return ProviderICloud }

func (*blockingCloudProvider) Download(context.Context, uuid.UUID, string, io.Writer) (int64, error) {
	panic("Download must not be reached")
}

func TestCloudImportCancelAndResumeKeepDurableReceipts(t *testing.T) {
	ctx := context.Background()
	catalogDirectory := t.TempDir()
	if err := os.Chmod(catalogDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDirectory, "catalog.sqlite3")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })
	if err := catalog.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	files := storage.NewRepositoryFSFactory(storage.NewRepositoryAccessCoordinator(), catalog.Queries)
	manager, err := storage.NewRepositoryManager(catalog.SQL, catalog.Queries, zap.NewNop(), nil, files)
	if err != nil {
		t.Fatal(err)
	}
	root, err := manager.EnsureDefaultRepositoryRoot(ctx, filepath.Join(t.TempDir(), "storage"))
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.CreateRepository(ctx, storage.CreateRepositorySpec{
		RequestID: "cloud-lifecycle-create", Actor: "test", Name: "Cloud target",
		DirectoryName: "cloud-target", Role: dbtypes.RepoRolePrimary, RootID: root.RootID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	owner, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username: "cloud-lifecycle-owner", Password: "test", DisplayName: "Owner", Role: "admin",
		WebauthnUserHandle: []byte("cloud-lifecycle-owner-handle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	credentialID := uuid.New()
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO cloud_credentials (
			credential_id, provider, display_name, identity_hash, masked_identity,
			owner_id, created_at, updated_at
		) VALUES (?, 'icloud', 'Test account', 'lifecycle-identity', 't***@example.com', ?, 1, 1)
	`, credentialID, owner.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpsertRepositoryCloudBinding(ctx, repo.UpsertRepositoryCloudBindingParams{
		RepositoryID: created.Repository.RepoID, CredentialID: credentialID,
		Provider: "icloud", OwnerID: owner.UserID, RemoteScope: dbtypes.JSON(`{"album":"Favorites"}`),
	}); err != nil {
		t.Fatal(err)
	}

	service := NewCloudSyncService(catalog.Queries, nil, nil, nil, "", t.TempDir(), zap.NewNop()).(*cloudSyncService)
	importer := &blockingCloudProvider{cancelObserved: make(chan struct{}), release: make(chan struct{})}
	service.registry = NewProviderRegistry(blockingCredentialProvider{importer: importer})
	access := CredentialAccess{UserID: owner.UserID}
	firstRunID, err := service.StartRepositoryImport(ctx, StartRepositoryImportInput{
		RepositoryID: created.Repository.RepoID, CredentialID: credentialID, Access: access,
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForCloudRunStatus(t, catalog.Queries, firstRunID, ImportRunStatusRunning)
	type cancelResult struct {
		run repo.CloudImportRun
		err error
	}
	cancelResultChannel := make(chan cancelResult, 1)
	go func() {
		run, cancelErr := service.CancelImportRun(ctx, firstRunID, access)
		cancelResultChannel <- cancelResult{run: run, err: cancelErr}
	}()
	select {
	case <-importer.cancelObserved:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not observe cancellation")
	}
	waitForCloudRunStatus(t, catalog.Queries, firstRunID, "cancelling")
	close(importer.release)
	result := <-cancelResultChannel
	cancelled, err := result.run, result.err
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled run status = %q", cancelled.Status)
	}
	storedCancelled, err := catalog.Queries.GetCloudImportRun(ctx, firstRunID)
	if err != nil || storedCancelled.Status != "cancelled" {
		t.Fatalf("durable cancelled run = %+v, error = %v", storedCancelled, err)
	}

	resumedRunID, err := service.ResumeImportRun(ctx, firstRunID, access)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := catalog.Queries.GetCloudImportRun(ctx, resumedRunID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ResumeOfRunID == nil || *resumed.ResumeOfRunID != firstRunID.String() {
		t.Fatalf("resume receipt predecessor = %+v, want %s", resumed.ResumeOfRunID, firstRunID)
	}
	if _, err := service.CancelImportRun(ctx, resumedRunID, access); err != nil {
		t.Fatal(err)
	}

	secondCredentialID := uuid.New()
	if _, err := catalog.SQL.ExecContext(ctx, `
		INSERT INTO cloud_credentials (
			credential_id, provider, display_name, identity_hash, masked_identity,
			owner_id, created_at, updated_at
		) VALUES (?, 'icloud', 'Second account', 'second-lifecycle-identity', 's***@example.com', ?, 1, 1)
	`, secondCredentialID, owner.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.UpsertRepositoryCloudBinding(ctx, repo.UpsertRepositoryCloudBindingParams{
		RepositoryID: created.Repository.RepoID, CredentialID: secondCredentialID,
		Provider: "icloud", OwnerID: owner.UserID, RemoteScope: dbtypes.JSON(`{"album":"Shared"}`),
	}); err != nil {
		t.Fatal(err)
	}
	status, err := service.GetRepositoryCloudStatus(ctx, created.Repository.RepoID, access)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Sources) != 2 {
		t.Fatalf("same-provider source count = %d, want 2", len(status.Sources))
	}
	scopesByCredential := make(map[uuid.UUID]string, len(status.Sources))
	for _, source := range status.Sources {
		scopesByCredential[source.Credential.CredentialID] = unmarshalRemoteScope(source.Binding.RemoteScope)["album"]
	}
	if scopesByCredential[credentialID] != "Favorites" || scopesByCredential[secondCredentialID] != "Shared" {
		t.Fatalf("same-provider source scopes = %v", scopesByCredential)
	}

	staleQueued, err := catalog.Queries.CreateCloudImportRun(ctx, repo.CreateCloudImportRunParams{
		RunID: uuid.New(), RepositoryID: created.Repository.RepoID, CredentialID: credentialID,
		Provider: "icloud", Status: ImportRunStatusQueued, OwnerID: owner.UserID,
	})
	if err != nil {
		t.Fatal(err)
	}
	staleCancelling, err := catalog.Queries.CreateCloudImportRun(ctx, repo.CreateCloudImportRunParams{
		RunID: uuid.New(), RepositoryID: created.Repository.RepoID, CredentialID: secondCredentialID,
		Provider: "icloud", Status: ImportRunStatusQueued, OwnerID: owner.UserID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.BeginCloudImportCancellation(ctx, staleCancelling.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.Queries.BeginRepositoryActivity(ctx, repo.BeginRepositoryActivityParams{
		RepoID: created.Repository.RepoID, Activity: dbtypes.RepositoryActivityImporting,
		UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.RecoverInterruptedRuns(ctx); err != nil {
		t.Fatal(err)
	}
	waitForCloudRunStatus(t, catalog.Queries, staleQueued.RunID, ImportRunStatusInterrupted)
	waitForCloudRunStatus(t, catalog.Queries, staleCancelling.RunID, "cancelled")
	repositoryAfterRecovery, err := catalog.Queries.GetRepository(ctx, created.Repository.RepoID)
	if err != nil || repositoryAfterRecovery.Activity != dbtypes.RepositoryActivityIdle {
		t.Fatalf("repository after startup recovery = %+v, error = %v", repositoryAfterRecovery, err)
	}
}

func waitForCloudRunStatus(t *testing.T, queries *repo.Queries, runID uuid.UUID, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		run, err := queries.GetCloudImportRun(context.Background(), runID)
		if err == nil && run.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, err := queries.GetCloudImportRun(context.Background(), runID)
	t.Fatalf("run status = %q, error = %v, want %q", run.Status, err, want)
}
