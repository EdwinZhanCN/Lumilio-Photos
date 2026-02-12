package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"server/internal/db/repo"
	"server/internal/storage/repocfg"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type fakePrimaryStorageRepositoryManager struct {
	getByPathRepos map[string]*repo.Repository
	addRepo        *repo.Repository
	initRepo       *repo.Repository

	getCalls  []string
	addCalls  []string
	initCalls []string
}

func (f *fakePrimaryStorageRepositoryManager) GetRepositoryByPath(path string) (*repo.Repository, error) {
	f.getCalls = append(f.getCalls, path)
	if r, ok := f.getByPathRepos[path]; ok && r != nil {
		return r, nil
	}
	return nil, fmt.Errorf("not found")
}

func (f *fakePrimaryStorageRepositoryManager) AddRepository(path string) (*repo.Repository, error) {
	f.addCalls = append(f.addCalls, path)
	if f.addRepo != nil {
		return f.addRepo, nil
	}
	return fakeRepo(path), nil
}

func (f *fakePrimaryStorageRepositoryManager) InitializeRepository(path string, _ repocfg.RepositoryConfig) (*repo.Repository, error) {
	f.initCalls = append(f.initCalls, path)
	if f.initRepo != nil {
		return f.initRepo, nil
	}
	return fakeRepo(path), nil
}

func fakeRepo(path string) *repo.Repository {
	repoUUID := uuid.New()
	return &repo.Repository{
		RepoID: pgtype.UUID{
			Bytes: repoUUID,
			Valid: true,
		},
		Name: "test-repo",
		Path: path,
	}
}

func writeRepositoryConfig(t *testing.T, repoPath string, name string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(repoPath, 0755))
	cfg := repocfg.NewRepositoryConfig(name)
	require.NoError(t, cfg.SaveConfigToFile(repoPath))
}

func TestResolvePrimaryStoragePaths(t *testing.T) {
	storageRoot, primaryRepoPath, err := resolvePrimaryStoragePaths(" ./data/storage ")
	require.NoError(t, err)

	expectedRoot, err := filepath.Abs(filepath.Clean("./data/storage"))
	require.NoError(t, err)
	require.Equal(t, expectedRoot, storageRoot)
	require.Equal(t, filepath.Join(expectedRoot, "primary"), primaryRepoPath)
}

func TestInitPrimaryStorage_InitializesPrimaryUnderStorageRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STORAGE_PATH", root)
	t.Setenv("STORAGE_STRATEGY", "date")
	t.Setenv("STORAGE_PRESERVE_FILENAME", "true")
	t.Setenv("STORAGE_DUPLICATE_HANDLING", "rename")

	manager := &fakePrimaryStorageRepositoryManager{
		getByPathRepos: map[string]*repo.Repository{},
	}

	err := initPrimaryStorage(manager)
	require.NoError(t, err)

	require.Empty(t, manager.addCalls)
	require.Len(t, manager.initCalls, 1)
	require.Equal(t, filepath.Join(root, "primary"), manager.initCalls[0])
}

func TestInitPrimaryStorage_RejectsLegacyRootRepository(t *testing.T) {
	root := t.TempDir()
	writeRepositoryConfig(t, root, "legacy-root")

	t.Setenv("STORAGE_PATH", root)
	t.Setenv("STORAGE_STRATEGY", "date")
	t.Setenv("STORAGE_PRESERVE_FILENAME", "true")
	t.Setenv("STORAGE_DUPLICATE_HANDLING", "rename")

	manager := &fakePrimaryStorageRepositoryManager{
		getByPathRepos: map[string]*repo.Repository{
			root: fakeRepo(root),
		},
	}

	err := initPrimaryStorage(manager)
	require.Error(t, err)
	require.Contains(t, err.Error(), "legacy repository detected")
	require.Empty(t, manager.addCalls)
	require.Empty(t, manager.initCalls)
}

func TestInitPrimaryStorage_RegistersExistingPrimaryRepository(t *testing.T) {
	root := t.TempDir()
	primaryPath := filepath.Join(root, "primary")
	writeRepositoryConfig(t, primaryPath, "primary")

	t.Setenv("STORAGE_PATH", root)
	t.Setenv("STORAGE_STRATEGY", "date")
	t.Setenv("STORAGE_PRESERVE_FILENAME", "true")
	t.Setenv("STORAGE_DUPLICATE_HANDLING", "rename")

	manager := &fakePrimaryStorageRepositoryManager{
		getByPathRepos: map[string]*repo.Repository{},
		addRepo:        fakeRepo(primaryPath),
	}

	err := initPrimaryStorage(manager)
	require.NoError(t, err)

	require.Len(t, manager.getCalls, 1)
	require.Equal(t, primaryPath, manager.getCalls[0])
	require.Len(t, manager.addCalls, 1)
	require.Equal(t, primaryPath, manager.addCalls[0])
	require.Empty(t, manager.initCalls)
}
