package handler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"server/config"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/db"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
)

func TestUploadJobStatusForCallerEnforcesOwnershipAndTerminalState(t *testing.T) {
	ctx, handler, encodedArgs := uploadStatusFixture(t, 7)
	row := &rivertype.JobRow{
		ID:          42,
		EncodedArgs: encodedArgs,
		State:       rivertype.JobStateDiscarded,
		Errors:      []rivertype.AttemptError{{Error: "materialization failed"}},
	}

	_, ok := handler.uploadJobStatusForCaller(ctx, row, 8)
	require.False(t, ok)

	status, ok := handler.uploadJobStatusForCaller(ctx, row, 7)
	require.True(t, ok)
	require.Equal(t, int64(42), status.TaskID)
	require.Equal(t, "photo.jpg", status.FileName)
	require.True(t, status.Terminal)
	require.False(t, status.Success)
	require.NotNil(t, status.Problem)
	require.Equal(t, problem.UploadProcessingFailed.Type, status.Problem.Type)
	require.Equal(t, problem.StableInstance(problem.UploadProcessingFailed, "river-job:42"), status.Problem.Instance)
}

func TestUploadJobStatusForCallerReportsRunningAsNonTerminal(t *testing.T) {
	ctx, handler, encodedArgs := uploadStatusFixture(t, 7)
	status, ok := handler.uploadJobStatusForCaller(ctx, &rivertype.JobRow{
		ID: 43, EncodedArgs: encodedArgs, State: rivertype.JobStateRunning,
	}, 7)
	require.True(t, ok)
	require.False(t, status.Terminal)
	require.False(t, status.Success)
}

func uploadStatusFixture(t *testing.T, ownerID int32) (context.Context, *AssetHandler, []byte) {
	t.Helper()
	ctx := context.Background()
	catalogDir := t.TempDir()
	require.NoError(t, os.Chmod(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(catalogDir, "catalog.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, catalog.Close(context.Background())) })
	require.NoError(t, catalog.Migrate(ctx))

	// This unit fixture isolates the staging-journal ownership lookup. The
	// repository and owner foreign keys are immaterial to that behavior.
	_, err = catalog.SQL.ExecContext(ctx, "PRAGMA foreign_keys = OFF")
	require.NoError(t, err)
	commitID := uuid.New()
	_, err = catalog.Queries.CreateRepositoryStagingCommit(ctx, repo.CreateRepositoryStagingCommitParams{
		CommitID: commitID, RepositoryID: uuid.New(), OwnerID: ownerID, SourceKind: "upload",
		StagingPath: "staging/photo.jpg", OriginalFilename: "photo.jpg", MimeType: "image/jpeg",
		FullHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		FileSize: 1, CreatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	require.NoError(t, err)
	encodedArgs, err := json.Marshal(jobs.IngestAssetArgs{CommitID: commitID})
	require.NoError(t, err)
	return ctx, &AssetHandler{queries: catalog.Queries}, encodedArgs
}

func TestAllRequestedUploadJobsTerminalRequiresFullCoverage(t *testing.T) {
	requested := []int64{1, 2, 3}
	partial := []dto.UploadJobStatusDTO{
		{TaskID: 1, Terminal: true, Success: true},
		{TaskID: 2, Terminal: true, Success: true},
	}
	require.False(t, allRequestedUploadJobsTerminal(requested, partial))

	complete := []dto.UploadJobStatusDTO{
		{TaskID: 1, Terminal: true, Success: true},
		{TaskID: 2, Terminal: true, Success: true},
		{TaskID: 3, Terminal: false, Success: false},
	}
	require.False(t, allRequestedUploadJobsTerminal(requested, complete))

	complete[2].Terminal = true
	require.True(t, allRequestedUploadJobsTerminal(requested, complete))
}
