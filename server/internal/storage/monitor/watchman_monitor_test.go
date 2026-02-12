package monitor

import (
	"os"
	"path/filepath"
	"testing"

	"server/internal/storage/repocfg"
	"server/internal/storage/watchman"

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

func TestShouldQueueDiscoveredPath(t *testing.T) {
	path, ok := shouldQueueDiscoveredPath("albums/2026/02/test.jpg")
	require.True(t, ok)
	require.Equal(t, "albums/2026/02/test.jpg", path)

	_, ok = shouldQueueDiscoveredPath("../escape/test.jpg")
	require.False(t, ok)

	_, ok = shouldQueueDiscoveredPath("inbox/2026/02/test.jpg")
	require.False(t, ok)

	_, ok = shouldQueueDiscoveredPath(".lumilio/assets/test.jpg")
	require.False(t, ok)

	_, ok = shouldQueueDiscoveredPath("albums/2026/02/test.txt")
	require.False(t, ok)
}

func TestIsWatchableRepositoryRoot(t *testing.T) {
	t.Run("missing path", func(t *testing.T) {
		require.False(t, isWatchableRepositoryRoot(filepath.Join(t.TempDir(), "not-exist")))
	})

	t.Run("directory without repo config", func(t *testing.T) {
		path := t.TempDir()
		require.False(t, isWatchableRepositoryRoot(path))
	})

	t.Run("valid repository root", func(t *testing.T) {
		path := t.TempDir()
		cfg := repocfg.NewRepositoryConfig("watchman-test")
		require.NoError(t, os.MkdirAll(path, 0755))
		require.NoError(t, cfg.SaveConfigToFile(path))
		require.True(t, isWatchableRepositoryRoot(path))
	})
}

func TestDiffRepositorySnapshots(t *testing.T) {
	previous := map[string]fileSnapshot{
		"albums/a.jpg": {Size: 10, MTimeMs: 100},
		"albums/b.jpg": {Size: 20, MTimeMs: 200},
	}
	current := map[string]fileSnapshot{
		"albums/b.jpg": {Size: 30, MTimeMs: 300},
		"albums/c.jpg": {Size: 40, MTimeMs: 400},
	}

	events := diffRepositorySnapshots(previous, current)
	require.Len(t, events, 3)

	require.Equal(t, "albums/a.jpg", events[0].Name)
	require.False(t, events[0].Exists)

	require.Equal(t, "albums/b.jpg", events[1].Name)
	require.True(t, events[1].Exists)
	require.Equal(t, int64(30), events[1].Size)
	require.Equal(t, int64(300), events[1].MTimeMs)

	require.Equal(t, "albums/c.jpg", events[2].Name)
	require.True(t, events[2].Exists)
	require.True(t, events[2].New)
}

func TestApplyFileEventsToSnapshot(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "albums"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "albums", "new.jpg"), []byte("hello"), 0644))

	snapshot := map[string]fileSnapshot{
		"albums/old.jpg": {Size: 5, MTimeMs: 50},
	}

	events := []watchman.FileEvent{
		{Name: "albums/new.jpg", Exists: true, Type: "f"},
		{Name: "albums/old.jpg", Exists: false, Type: "f"},
		{Name: "inbox/skip.jpg", Exists: true, Type: "f", Size: 1, MTimeMs: 1},
	}

	applyFileEventsToSnapshot(root, snapshot, events)

	require.NotContains(t, snapshot, "albums/old.jpg")
	newEntry, ok := snapshot["albums/new.jpg"]
	require.True(t, ok)
	require.Equal(t, int64(5), newEntry.Size)
	require.Greater(t, newEntry.MTimeMs, int64(0))
	require.NotContains(t, snapshot, "inbox/skip.jpg")
}

func TestSnapshotRepositoryFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".lumilio", "assets"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "inbox"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "albums"), 0755))

	require.NoError(t, os.WriteFile(filepath.Join(root, ".lumilio", "assets", "ignored.jpg"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "inbox", "ignored.jpg"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "albums", "keep.jpg"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "albums", "skip.txt"), []byte("x"), 0644))

	snapshot, err := snapshotRepositoryFiles(root)
	require.NoError(t, err)
	require.Len(t, snapshot, 1)

	entry, ok := snapshot["albums/keep.jpg"]
	require.True(t, ok)
	require.Equal(t, int64(1), entry.Size)
	require.Greater(t, entry.MTimeMs, int64(0))
}
