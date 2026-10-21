package storage

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/repocfg"
)

type rejectingLocks struct {
	path string
	err  error
}

func (l *rejectingLocks) Acquire(_ context.Context, p string, _ bool) (func(), error) {
	l.path = p
	return nil, l.err
}
func (l *rejectingLocks) Inspect(string, string) (RepositoryLockInfo, error) {
	return RepositoryLockInfo{}, l.err
}
func TestRuntimeOwnershipUsesInstalledLockProvider(t *testing.T) {
	rejection := errors.New("alternative ownership refused")
	locks := &rejectingLocks{err: rejection}
	rm := &DefaultRepositoryManager{ownershipOn: true, ownership: map[string]func(){}}
	rm.SetLockProvider(locks)
	path := t.TempDir()
	if err := rm.claimRuntimeStoragePath(context.Background(), "repository", path); !errors.Is(err, rejection) {
		t.Fatalf("got %v", err)
	}
	if locks.path != filepath.Join(path, ".lumiliorepo.lock") {
		t.Fatal(locks.path)
	}
	if _, err := os.Stat(locks.path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("native lock created despite alternative provider")
	}
}

type rejectingIdentity struct {
	LocalRepositoryIdentityDetector
	err error
}

func (i rejectingIdentity) VerifyMarker([]byte, uuid.UUID) error { return i.err }
func TestRootedOpenUsesInstalledIdentityDetector(t *testing.T) {
	path := t.TempDir()
	config := repocfg.NewRepositoryConfig("identity")
	if err := config.SaveConfigToFile(path); err != nil {
		t.Fatal(err)
	}
	factory := NewRepositoryFSFactory(nil, nil)
	rejection := errors.New("alternative identity refused")
	factory.SetIdentityDetector(rejectingIdentity{err: rejection})
	opened, err := factory.Open(repo.Repository{RepoID: uuid.MustParse(config.ID), Path: path, Reachability: dbtypes.RepositoryReachabilityActive})
	if opened != nil || !errors.Is(err, rejection) {
		t.Fatalf("open=%v err=%v", opened, err)
	}
}

type statErrorObserver struct {
	OSStorageObserver
	err error
}

func (o statErrorObserver) Stat(string) (fs.FileInfo, error) { return nil, o.err }
func TestValidationInjectedStatErrors(t *testing.T) {
	for _, err := range []error{fs.ErrPermission, errors.New("I/O failure"), nil, fs.ErrNotExist} {
		rm := &DefaultRepositoryManager{logger: zap.NewNop()}
		rm.SetObservationBackend(statErrorObserver{err: err})
		got, validationErr := rm.validateRepository(t.TempDir())
		if validationErr != nil || got.Valid || len(got.Errors) != 1 {
			t.Fatalf("%+v %v", got, validationErr)
		}
		expected := "unknown"
		if errors.Is(err, fs.ErrPermission) {
			expected = "permission_denied"
		}
		if errors.Is(err, fs.ErrNotExist) {
			expected = "does not exist"
		}
		if !strings.Contains(got.Errors[0], expected) {
			t.Fatalf("%s missing %s", got.Errors[0], expected)
		}
	}
}
