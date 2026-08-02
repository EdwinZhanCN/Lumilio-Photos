package lumen

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrOwnerBusy = errors.New("Lumen supervisor owner lock is busy")

// OwnerLock is held by the supervision root for the complete process-tree
// lifetime. It is intentionally separate from a diagnostic PID file: a new
// Desktop process may only proceed after the OS lock is acquired.
type OwnerLock struct {
	file  *os.File
	path  string
	token string
}

func AcquireOwnerLock(path string) (*OwnerLock, error) {
	if path == "" {
		return nil, errors.New("Lumen owner lock path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockOwnerFile(file); err != nil {
		_ = file.Close()
		if errors.Is(err, ErrOwnerBusy) {
			return nil, err
		}
		return nil, fmt.Errorf("lock Lumen owner file: %w", err)
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		_ = unlockOwnerFile(file)
		_ = file.Close()
		return nil, err
	}
	token := hex.EncodeToString(raw[:])
	if err := file.Truncate(0); err != nil {
		_ = unlockOwnerFile(file)
		_ = file.Close()
		return nil, err
	}
	if _, err := file.WriteString(token + "\n"); err != nil {
		_ = unlockOwnerFile(file)
		_ = file.Close()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		_ = unlockOwnerFile(file)
		_ = file.Close()
		return nil, err
	}
	return &OwnerLock{file: file, path: path, token: token}, nil
}

func (l *OwnerLock) Token() string {
	if l == nil {
		return ""
	}
	return l.token
}

func (l *OwnerLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlockOwnerFile(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if err != nil {
		return err
	}
	return closeErr
}
