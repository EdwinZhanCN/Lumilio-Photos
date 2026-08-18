package handler

import (
	"testing"

	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"

	"server/internal/api/dto"
	"server/internal/api/problem"
)

func TestUploadJobStatusForCallerEnforcesOwnershipAndTerminalState(t *testing.T) {
	row := &rivertype.JobRow{
		ID:          42,
		EncodedArgs: []byte(`{"userId":"7","fileName":"photo.jpg"}`),
		State:       rivertype.JobStateDiscarded,
		Errors:      []rivertype.AttemptError{{Error: "materialization failed"}},
	}

	_, ok := uploadJobStatusForCaller(row, "8")
	require.False(t, ok)

	status, ok := uploadJobStatusForCaller(row, "7")
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
	status, ok := uploadJobStatusForCaller(&rivertype.JobRow{
		ID:          43,
		EncodedArgs: []byte(`{"userId":"anonymous","fileName":"photo.jpg"}`),
		State:       rivertype.JobStateRunning,
	}, "anonymous")

	require.True(t, ok)
	require.False(t, status.Terminal)
	require.False(t, status.Success)
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
