package marker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrIdentityChanged = errors.New("marker identity changed")

// Guard validates the complete current marker against the expected identity.
// Missing markers are supplied as nil. Corrupt/newer markers must be refused.
type Guard func(current []byte) error

type TempFile interface {
	io.Writer
	Name() string
	Chmod(fs.FileMode) error
	Sync() error
	Close() error
}

// AtomicFS is intentionally separate from the read-only observer. Implementations
// must provide an atomic, same-filesystem Rename; failure hooks are testable.
type AtomicFS interface {
	Reader
	Lstat(string) (fs.FileInfo, error)
	CreateTemp(string, string) (TempFile, error)
	Rename(string, string) error
	Remove(string) error
	SyncDir(string) error
}
type OSAtomicFS struct{}

func (OSAtomicFS) Lstat(p string) (fs.FileInfo, error) { return os.Lstat(p) }

func (OSAtomicFS) ReadFile(p string) ([]byte, error)              { return os.ReadFile(p) }
func (OSAtomicFS) CreateTemp(p, pattern string) (TempFile, error) { return os.CreateTemp(p, pattern) }
func (OSAtomicFS) Rename(a, b string) error                       { return replaceFile(a, b) }
func (OSAtomicFS) Remove(p string) error                          { return os.Remove(p) }
func (OSAtomicFS) SyncDir(p string) error                         { return syncDir(p) }

// WriteAtomic writes a complete marker, syncs it, checks the guard again, then
// renames and syncs its directory. Non-regular marker entries are refused;
// data must be a validated complete encoding (the config SaveGuarded methods
// supply it). A directory-sync failure can mean the new
// marker is already visible; callers must re-read instead of blindly rolling back.
// The caller serializes cooperating writers through its mutation/ownership lease.
func WriteAtomic(files AtomicFS, path string, data []byte, guard Guard) error {
	if guard == nil {
		return fmt.Errorf("marker guard is required")
	}
	read := func() ([]byte, error) {
		info, err := files.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if info == nil || !info.Mode().IsRegular() {
			return nil, ErrIdentityChanged
		}
		// A dangling link or vanished entry is not permission to replace identity.
		return files.ReadFile(path)
	}
	old, err := read()
	if err != nil {
		return err
	}
	if err := guard(old); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	temp, err := files.CreateTemp(dir, ".lumilio-marker-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer files.Remove(name)
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
	}()
	if err := temp.Chmod(0644); err != nil {
		return err
	}
	n, err := temp.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	err = temp.Close()
	closed = true
	if err != nil {
		return err
	}
	current, err := read()
	if err != nil {
		return err
	}
	if err := guard(current); err != nil {
		return err
	}
	// Also retain complete fields if an external writer changed this identity.
	if !bytes.Equal(old, current) {
		return ErrIdentityChanged
	}
	if err := files.Rename(name, path); err != nil {
		return err
	}
	return files.SyncDir(dir)
}
