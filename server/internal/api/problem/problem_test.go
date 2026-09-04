package problem

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStableProblemReferencesAreOpaqueAndRepeatable(t *testing.T) {
	const operationID = "restore-operation-contains-private-id"
	first := ReferenceFor(BackupRestoreFailed, operationID, false)
	second := ReferenceFor(BackupRestoreFailed, operationID, false)
	otherOperation := ReferenceFor(BackupRestoreFailed, operationID+"-other", false)
	otherType := ReferenceFor(RepositoryScanFailed, operationID, false)

	require.Equal(t, first, second)
	require.NotEqual(t, first.Instance, otherOperation.Instance)
	require.NotEqual(t, first.Instance, otherType.Instance)
	require.Regexp(t, `^urn:lumilio:problem:[0-9a-f]{32}$`, first.Instance)
	require.False(t, strings.Contains(first.Instance, operationID))
	require.Equal(t, BackupRestoreFailed.Type, first.Type)
	require.NotNil(t, first.Retryable)
	require.False(t, *first.Retryable)
}

func TestRegisteredProblemCatalogIsCompleteAndUnique(t *testing.T) {
	seen := make(map[string]struct{})
	for _, descriptor := range Registered() {
		require.NotEmpty(t, descriptor.Type)
		require.NotEmpty(t, descriptor.Title)
		require.NotEmpty(t, descriptor.Definition)
		require.NotEmpty(t, descriptor.Recovery)
		require.GreaterOrEqual(t, descriptor.Status, 400)
		require.LessOrEqual(t, descriptor.Status, 599)
		_, duplicate := seen[descriptor.Type]
		require.Falsef(t, duplicate, "duplicate Problem type %s", descriptor.Type)
		seen[descriptor.Type] = struct{}{}
	}
	require.Len(t, seen, 24)
}
