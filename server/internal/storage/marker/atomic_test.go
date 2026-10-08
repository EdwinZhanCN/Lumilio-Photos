package marker_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"server/internal/storage/marker"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
	"server/internal/storage/testfixture"
)

var injected = errors.New("injected marker failure")

type failingFS struct {
	marker.OSAtomicFS
	stage   string
	replace func()
}

type nonRegularInfo struct{ fs.FileInfo }

func (nonRegularInfo) Mode() fs.FileMode { return fs.ModeSymlink }
func (f failingFS) Lstat(path string) (fs.FileInfo, error) {
	if f.stage == "lstat" {
		return nil, injected
	}
	if f.stage == "nil-stat" {
		return nil, nil
	}
	info, err := f.OSAtomicFS.Lstat(path)
	if f.stage == "nonregular" {
		return nonRegularInfo{info}, err
	}
	return info, err
}
func (f failingFS) ReadFile(path string) ([]byte, error) {
	if f.stage == "read" {
		return nil, injected
	}
	return f.OSAtomicFS.ReadFile(path)
}
func (f failingFS) CreateTemp(dir, pattern string) (marker.TempFile, error) {
	if f.stage == "create" {
		return nil, injected
	}
	temp, err := f.OSAtomicFS.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	return failingTemp{TempFile: temp, stage: f.stage, replace: f.replace}, nil
}
func (f failingFS) Rename(a, b string) error {
	if f.stage == "rename" {
		return injected
	}
	return f.OSAtomicFS.Rename(a, b)
}
func (f failingFS) SyncDir(p string) error {
	if f.stage == "dir-sync" {
		return injected
	}
	return f.OSAtomicFS.SyncDir(p)
}

type failingTemp struct {
	marker.TempFile
	stage   string
	replace func()
}

func (f failingTemp) Chmod(mode fs.FileMode) error {
	if f.stage == "chmod" {
		return injected
	}
	return f.TempFile.Chmod(mode)
}
func (f failingTemp) Write(data []byte) (int, error) {
	if f.stage == "write" {
		n, _ := f.TempFile.Write(data[:len(data)/2])
		return n, injected
	}
	if f.stage == "short-write" {
		return f.TempFile.Write(data[:len(data)/2])
	}
	return f.TempFile.Write(data)
}
func (f failingTemp) Sync() error {
	if f.stage == "sync" {
		return injected
	}
	return f.TempFile.Sync()
}
func (f failingTemp) Close() error {
	err := f.TempFile.Close()
	if f.replace != nil {
		f.replace()
	}
	if f.stage == "close" {
		return injected
	}
	return err
}

func TestAtomicMarkerFailuresLeaveCompleteOldOrNew(t *testing.T) {
	for _, kind := range []string{"repository", "location"} {
		for _, stage := range []string{"lstat", "read", "nil-stat", "nonregular", "create", "chmod", "write", "short-write", "sync", "close", "rename", "dir-sync", "success"} {
			t.Run(kind+"/"+stage, func(t *testing.T) {
				location := testfixture.NewLocation(t, testfixture.Valid)
				repository := location.Repository(t, "child", testfixture.Valid, testfixture.Full)
				path := filepath.Join(repository.Path, ".lumiliorepo")
				if kind == "location" {
					path = filepath.Join(location.Path, rootcfg.FileName)
				}
				old, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "repository" {
					config := repository.Config
					config.Name = "new"
					err = config.SaveGuarded(failingFS{stage: stage}, repository.Path, config.ID)
				} else {
					config := location.Config
					config.Name = "new"
					err = config.SaveGuarded(failingFS{stage: stage}, location.Path, config.ID)
				}
				if (stage == "success") != (err == nil) {
					t.Fatalf("stage %s: %v", stage, err)
				}
				current, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				name := ""
				if kind == "repository" {
					config, err := repocfg.ParseConfig(current)
					if err != nil {
						t.Fatal(err)
					}
					name = config.Name
				} else {
					reading := rootcfg.DecodeMarker(current)
					if reading.State != marker.Valid {
						t.Fatal(reading)
					}
					name = reading.Config.Name
				}
				if stage == "success" || stage == "dir-sync" {
					if name != "new" {
						t.Fatal("new marker missing")
					}
				} else if string(old) != string(current) {
					t.Fatal("failed write changed old marker")
				}
				temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".lumilio-marker-*"))
				if err != nil || len(temps) != 0 {
					t.Fatalf("temp files left: %v %v", temps, err)
				}
			})
		}
	}
}

func TestGuardedMarkersRefuseExternalIdentityAndRollback(t *testing.T) {
	for _, kind := range []string{"repository", "location"} {
		t.Run(kind, func(t *testing.T) {
			location := testfixture.NewLocation(t, testfixture.Valid)
			repository := location.Repository(t, "child", testfixture.Valid, testfixture.Full)
			path := filepath.Join(repository.Path, ".lumiliorepo")
			if kind == "location" {
				path = filepath.Join(location.Path, rootcfg.FileName)
			}
			replacement := func() {
				if kind == "repository" {
					config := repository.Config
					config.ID = "55555555-5555-4555-8555-555555555555"
					if err := config.SaveConfigToFile(repository.Path); err != nil {
						t.Fatal(err)
					}
				} else {
					config := location.Config
					config.ID = "55555555-5555-4555-8555-555555555555"
					if err := config.Save(location.Path); err != nil {
						t.Fatal(err)
					}
				}
			}
			var err error
			if kind == "repository" {
				config := repository.Config
				config.Name = "rename"
				err = config.SaveGuarded(failingFS{replace: replacement}, repository.Path, repository.Config.ID)
			} else {
				config := location.Config
				config.Name = "rename"
				err = config.SaveGuarded(failingFS{replace: replacement}, location.Path, location.Config.ID)
			}
			if !errors.Is(err, marker.ErrIdentityChanged) {
				t.Fatal(err)
			}
			replaced, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "repository" {
				err = repository.Config.SaveGuarded(marker.OSAtomicFS{}, repository.Path, repository.Config.ID)
			} else {
				err = location.Config.SaveGuarded(marker.OSAtomicFS{}, location.Path, location.Config.ID)
			}
			if !errors.Is(err, marker.ErrIdentityChanged) {
				t.Fatalf("rollback clobbered identity: %v", err)
			}
			current, err := os.ReadFile(path)
			if err != nil || string(current) != string(replaced) {
				t.Fatal("replacement not preserved")
			}
		})
	}
}

func TestGuardedMarkerCreationAndInvalidMarkerPreservation(t *testing.T) {
	for _, state := range []testfixture.Marker{testfixture.Missing, testfixture.Corrupt, testfixture.Unsupported} {
		location := testfixture.NewLocation(t, state)
		repository := location.Repository(t, "child", state, testfixture.Full)
		rootPath := filepath.Join(location.Path, rootcfg.FileName)
		repoPath := filepath.Join(repository.Path, ".lumiliorepo")
		rootOld, _ := os.ReadFile(rootPath)
		repoOld, _ := os.ReadFile(repoPath)
		rootErr := location.Config.SaveGuarded(marker.OSAtomicFS{}, location.Path, "")
		repoErr := repository.Config.SaveGuarded(marker.OSAtomicFS{}, repository.Path, "")
		if state == testfixture.Missing {
			if rootErr != nil || repoErr != nil {
				t.Fatalf("creation: %v %v", rootErr, repoErr)
			}
		} else {
			if !errors.Is(rootErr, marker.ErrIdentityChanged) || !errors.Is(repoErr, marker.ErrIdentityChanged) {
				t.Fatalf("invalid markers allowed: %v %v", rootErr, repoErr)
			}
			rootNew, _ := os.ReadFile(rootPath)
			repoNew, _ := os.ReadFile(repoPath)
			if string(rootOld) != string(rootNew) || string(repoOld) != string(repoNew) {
				t.Fatal("invalid bytes changed")
			}
		}
	}
}
