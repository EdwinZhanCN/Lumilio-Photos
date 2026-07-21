package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"server/platform/fsprivacy"
)

func main() {
	if len(os.Args) != 2 || strings.TrimSpace(os.Args[1]) == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/secretinit <secret-file>")
		os.Exit(2)
	}
	if err := ensureSecret(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "initialize secret: %v\n", err)
		os.Exit(1)
	}
}

func ensureSecret(path string) error {
	path = filepath.Clean(path)
	if data, err := os.ReadFile(path); err == nil {
		if strings.TrimSpace(string(data)) == "" {
			return errors.New("existing secret file is empty")
		}
		if info, statErr := os.Stat(path); statErr != nil {
			return statErr
		} else if info.Mode().Perm() == 0o600 {
			return nil
		}
		return fsprivacy.ApplyFileMode(path, 0o600)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := fsprivacy.ApplyDirectoryMode(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return ensureSecret(path)
	}
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, hex.EncodeToString(random)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return fsprivacy.ApplyFileMode(path, 0o600)
}
