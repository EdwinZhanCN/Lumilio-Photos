// Package resources verifies and materialises the immutable tool payload
// shipped with Desktop. It never searches PATH and never overwrites a current
// version directory.
package resources

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"desktop/internal/platform"
)

const SchemaVersion = 1

type Manifest struct {
	SchemaVersion int     `json:"schemaVersion"`
	Entries       []Entry `json:"entries"`
}

type Entry struct {
	LogicalName string `json:"logicalName"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	Version     string `json:"version"`
	SHA256      string `json:"sha256"`
	Mode        uint32 `json:"mode"`
	Path        string `json:"path"`
}

func Load(data []byte) (Manifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode resource manifest: %w", err)
	}
	if manifest.SchemaVersion != SchemaVersion {
		return Manifest{}, fmt.Errorf("unsupported resource manifest schema version %d", manifest.SchemaVersion)
	}
	for _, entry := range manifest.Entries {
		if err := validateEntry(entry); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, nil
}

func Verify(root string, manifest Manifest) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		if entry.Platform != runtime.GOOS || entry.Arch != runtime.GOARCH {
			continue
		}
		path, err := safeJoin(root, entry.Path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("resource %s is unavailable: %w", entry.LogicalName, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("resource %s is not a regular file", entry.LogicalName)
		}
		if err := verifyFile(path, entry.SHA256); err != nil {
			return fmt.Errorf("resource %s failed verification: %w", entry.LogicalName, err)
		}
		if entry.Mode != 0 && !fileModeMatches(info.Mode(), os.FileMode(entry.Mode)) {
			return fmt.Errorf("resource %s has mode %o, want %o", entry.LogicalName, info.Mode().Perm(), entry.Mode)
		}
	}
	return nil
}

func Materialize(sourceRoot string, destinationRoot string, manifest Manifest) error {
	if err := Verify(sourceRoot, manifest); err != nil {
		return err
	}
	if err := platform.EnsurePrivateDirectory(destinationRoot); err != nil {
		return err
	}
	for _, entry := range manifest.Entries {
		if entry.Platform != runtime.GOOS || entry.Arch != runtime.GOARCH {
			continue
		}
		source, err := safeJoin(sourceRoot, entry.Path)
		if err != nil {
			return err
		}
		target, err := safeJoin(destinationRoot, entry.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("refusing to overwrite materialized resource %s", entry.LogicalName)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		input, err := os.Open(source)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(entry.Mode))
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeOutputErr := output.Close()
		closeInputErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
	}
	return Verify(destinationRoot, manifest)
}

func validateEntry(entry Entry) error {
	if strings.TrimSpace(entry.LogicalName) == "" || strings.TrimSpace(entry.Path) == "" || strings.TrimSpace(entry.Version) == "" {
		return errors.New("resource manifest entry is incomplete")
	}
	if strings.TrimSpace(entry.Platform) == "" || strings.TrimSpace(entry.Arch) == "" {
		return fmt.Errorf("resource %s has no platform or architecture", entry.LogicalName)
	}
	if len(entry.SHA256) != sha256.Size*2 {
		return fmt.Errorf("resource %s has invalid SHA-256", entry.LogicalName)
	}
	if _, err := hex.DecodeString(entry.SHA256); err != nil {
		return fmt.Errorf("resource %s has invalid SHA-256: %w", entry.LogicalName, err)
	}
	if entry.Mode == 0 {
		return fmt.Errorf("resource %s has no file mode", entry.LogicalName)
	}
	return nil
}

func safeJoin(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("resource path must be relative: %s", relative)
	}
	joined := filepath.Clean(filepath.Join(root, relative))
	cleanRoot := filepath.Clean(root)
	if joined != cleanRoot && !strings.HasPrefix(joined, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("resource path escapes root: %s", relative)
	}
	return joined, nil
}

func verifyFile(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("sha256 %s does not match %s", actual, expected)
	}
	return nil
}
