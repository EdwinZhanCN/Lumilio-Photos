package monitor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanRelativePath(t *testing.T) {
	path, ok := cleanRelativePath("2026/02/test.jpg")
	require.True(t, ok)
	require.Equal(t, "2026/02/test.jpg", path)

	_, ok = cleanRelativePath("../escape.jpg")
	require.False(t, ok)

	_, ok = cleanRelativePath("/abs/path.jpg")
	require.False(t, ok)
}

func TestJoinWatchRelative(t *testing.T) {
	require.Equal(t, "repo/inbox", joinWatchRelative("repo", "inbox"))
	require.Equal(t, "inbox", joinWatchRelative("", "inbox"))
	require.Equal(t, "repo/inbox", joinWatchRelative("repo", "", "inbox"))
}

func TestIsExcludedWorkspacePath(t *testing.T) {
	require.True(t, isExcludedWorkspacePath(".lumilio"))
	require.True(t, isExcludedWorkspacePath(".lumilio/assets/1.jpg"))
	require.True(t, isExcludedWorkspacePath("inbox"))
	require.True(t, isExcludedWorkspacePath("inbox/2026/02/a.jpg"))
	require.False(t, isExcludedWorkspacePath("albums/2026/02/a.jpg"))
	require.False(t, isExcludedWorkspacePath("workspace/raw/IMG_0001.CR3"))
}
