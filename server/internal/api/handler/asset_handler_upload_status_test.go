package handler

import (
	"testing"

	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/require"
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
	require.NotNil(t, status.Error)
	require.Equal(t, "materialization failed", *status.Error)
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
