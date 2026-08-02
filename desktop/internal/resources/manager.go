package resources

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"desktop/internal/platform"
)

const installSchemaVersion = 1

// Pointer is the only authoritative reference to the materialised resource
// tree. The directory is content-addressed by the signed manifest digest and
// is never overwritten in place.
type Pointer struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Version        string `json:"version"`
	Platform       string `json:"platform"`
	Arch           string `json:"arch"`
	ManifestSHA256 string `json:"manifestSHA256"`
	Directory      string `json:"directory"`
}

type journal struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Phase          string `json:"phase"`
	Version        string `json:"version"`
	Platform       string `json:"platform"`
	Arch           string `json:"arch"`
	ManifestSHA256 string `json:"manifestSHA256"`
	Directory      string `json:"directory"`
	Staging        string `json:"staging,omitempty"`
}

// Manager owns the packaged-resource transaction. Source is an embedded
// filesystem rooted at the Desktop binary; it is never replaced by PATH or a
// user-controlled directory.
type Manager struct {
	paths      platform.Paths
	source     fs.FS
	sourceRoot string
	manifest   Manifest
	digest     string
}

func NewManager(paths platform.Paths, source fs.FS, sourceRoot string, manifest Manifest) *Manager {
	return &Manager{
		paths: paths, source: source, sourceRoot: strings.Trim(sourceRoot, "/"),
		manifest: manifest, digest: manifestDigest(manifest),
	}
}

// Ensure reconciles any interrupted transaction and installs the current
// platform payload if the pointer is missing or no longer matches the
// embedded manifest. Empty manifests are valid for development builds.
func (m *Manager) Ensure() error {
	if m == nil {
		return errors.New("resource manager is unavailable")
	}
	if err := m.Reconcile(); err != nil {
		return err
	}
	entries := m.platformEntries()
	if len(entries) == 0 {
		return nil
	}
	current, err := m.loadPointer()
	if err == nil && current.ManifestSHA256 == m.digest && m.verifyDirectory(current.Directory) == nil {
		return nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// A malformed pointer is a recovery condition, not a reason to guess
		// which version directory should be used.
		return err
	}

	version := safeSegment(entries[0].Version)
	directory := version + "-" + runtime.GOOS + "-" + runtime.GOARCH + "-" + strings.TrimPrefix(m.digest, "sha256:")[:12]
	target, err := managerSafeJoin(m.paths.ResourcesVersions, directory)
	if err != nil {
		return err
	}
	if err := m.verifyDirectory(directory); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			if removeErr := os.RemoveAll(target); removeErr != nil {
				return fmt.Errorf("resource target is invalid: %w (remove invalid target: %v)", err, removeErr)
			}
		}
		if err := m.materialize(directory, target); err != nil {
			return err
		}
	}
	return m.commit(directory, entries[0].Version)
}

// Reconcile resumes only the journal phase that is durably recorded. An
// interrupted staging tree is disposable; verified/promoting/committed trees
// are re-verified before pointer completion.
func (m *Manager) Reconcile() error {
	data, err := os.ReadFile(m.paths.ResourcesInstall)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read resource install journal: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var item journal
	if err := decoder.Decode(&item); err != nil {
		return fmt.Errorf("decode resource install journal: %w", err)
	}
	if item.SchemaVersion != installSchemaVersion || item.Version == "" || !validDigest(item.ManifestSHA256) || item.Directory == "" {
		return errors.New("invalid resource install journal")
	}
	target, err := managerSafeJoin(m.paths.ResourcesVersions, item.Directory)
	if err != nil {
		return err
	}
	staging, err := managerSafeJoin(m.paths.ResourcesVersions, item.Staging)
	if item.Staging != "" && err != nil {
		return err
	}
	switch item.Phase {
	case "staging":
		if item.Staging != "" {
			if err := os.RemoveAll(staging); err != nil {
				return err
			}
		}
		if err := os.Remove(m.paths.ResourcesInstall); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	case "verified":
		if _, statErr := os.Stat(target); errors.Is(statErr, fs.ErrNotExist) {
			if item.Staging == "" {
				return errors.New("verified resource journal has no staging directory")
			}
			if err := m.verifyPath(staging); err != nil {
				return fmt.Errorf("verified resource staging did not survive restart: %w", err)
			}
			if err := m.promote(item, staging, target); err != nil {
				return err
			}
		} else if statErr != nil {
			return statErr
		} else if err := m.verifyDirectory(item.Directory); err != nil {
			return fmt.Errorf("verified resource target failed verification: %w", err)
		}
		if item.Staging != "" {
			if err := os.RemoveAll(staging); err != nil {
				return err
			}
		}
		if err := m.writeJournal(journal{Phase: "promoting", Version: item.Version, Platform: item.Platform, Arch: item.Arch, ManifestSHA256: item.ManifestSHA256, Directory: item.Directory}); err != nil {
			return err
		}
		return m.writePointerAndFinish(item)
	case "promoting":
		if _, statErr := os.Stat(target); errors.Is(statErr, fs.ErrNotExist) && item.Staging != "" {
			if err := m.verifyPath(staging); err != nil {
				return fmt.Errorf("resource promotion has no verified target: %w", err)
			}
			if err := m.promote(item, staging, target); err != nil {
				return err
			}
		}
		if err := m.verifyDirectory(item.Directory); err != nil {
			return fmt.Errorf("promoting resource tree failed verification: %w", err)
		}
		return m.writePointerAndFinish(item)
	case "committed":
		pointer, pointerErr := m.loadPointer()
		if pointerErr != nil || pointer.ManifestSHA256 != item.ManifestSHA256 || pointer.Directory != item.Directory || m.verifyDirectory(item.Directory) != nil {
			return errors.New("committed resource journal does not match a verified pointer")
		}
		return os.Remove(m.paths.ResourcesInstall)
	default:
		return fmt.Errorf("unsupported resource install phase %q", item.Phase)
	}
}

func (m *Manager) materialize(directory, target string) error {
	token := strings.TrimPrefix(m.digest, "sha256:")[:16]
	staging := filepath.Join(m.paths.ResourcesVersions, ".staging-"+token)
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	if err := platform.EnsurePrivateDirectory(staging); err != nil {
		return err
	}
	stagingName := filepath.Base(staging)
	item := journal{Phase: "staging", Version: m.platformEntries()[0].Version, Platform: runtime.GOOS, Arch: runtime.GOARCH, ManifestSHA256: m.digest, Directory: directory, Staging: stagingName}
	if err := m.writeJournal(item); err != nil {
		return err
	}
	if err := materializeFS(m.source, m.sourceRoot, staging, m.manifest); err != nil {
		return err
	}
	if err := m.verifyPath(staging); err != nil {
		return err
	}
	item.Phase = "verified"
	if err := m.writeJournal(item); err != nil {
		return err
	}
	if err := os.Remove(target); err == nil {
		return errors.New("refusing to replace an existing resource directory")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(staging, target); err != nil {
		return err
	}
	item.Phase = "promoting"
	item.Staging = ""
	if err := m.writeJournal(item); err != nil {
		return err
	}
	return nil
}

func (m *Manager) promote(item journal, staging, target string) error {
	if _, err := os.Lstat(target); err == nil {
		if verifyErr := m.verifyDirectory(item.Directory); verifyErr != nil {
			return fmt.Errorf("refusing to replace an invalid resource target: %w", verifyErr)
		}
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(staging, target); err != nil {
		return err
	}
	return nil
}

func (m *Manager) commit(directory, version string) error {
	item := journal{Phase: "promoting", Version: version, Platform: runtime.GOOS, Arch: runtime.GOARCH, ManifestSHA256: m.digest, Directory: directory}
	if err := m.writePointerAndFinish(item); err != nil {
		return err
	}
	return nil
}

func (m *Manager) writePointerAndFinish(item journal) error {
	pointer := Pointer{SchemaVersion: installSchemaVersion, Version: item.Version, Platform: item.Platform, Arch: item.Arch, ManifestSHA256: item.ManifestSHA256, Directory: item.Directory}
	data, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return err
	}
	if err := platform.WriteAtomic(m.paths.ResourcesCurrent, append(data, '\n'), 0o600); err != nil {
		return err
	}
	if err := m.writeJournal(journal{Phase: "committed", Version: item.Version, Platform: item.Platform, Arch: item.Arch, ManifestSHA256: item.ManifestSHA256, Directory: item.Directory}); err != nil {
		return err
	}
	return os.Remove(m.paths.ResourcesInstall)
}

func (m *Manager) writeJournal(item journal) error {
	item.SchemaVersion = installSchemaVersion
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteAtomic(m.paths.ResourcesInstall, append(data, '\n'), 0o600)
}

func (m *Manager) loadPointer() (Pointer, error) {
	data, err := os.ReadFile(m.paths.ResourcesCurrent)
	if err != nil {
		return Pointer{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var pointer Pointer
	if err := decoder.Decode(&pointer); err != nil {
		return Pointer{}, err
	}
	if pointer.SchemaVersion != installSchemaVersion || pointer.Version == "" || pointer.Platform == "" || pointer.Arch == "" || !validDigest(pointer.ManifestSHA256) || pointer.Directory == "" {
		return Pointer{}, errors.New("invalid resource current pointer")
	}
	if _, err := managerSafeJoin(m.paths.ResourcesVersions, pointer.Directory); err != nil {
		return Pointer{}, err
	}
	return pointer, nil
}

func (m *Manager) verifyDirectory(directory string) error {
	target, err := managerSafeJoin(m.paths.ResourcesVersions, directory)
	if err != nil {
		return err
	}
	return m.verifyPath(target)
}

func (m *Manager) verifyPath(root string) error {
	return Verify(root, m.manifest)
}

func (m *Manager) platformEntries() []Entry {
	entries := make([]Entry, 0)
	for _, entry := range m.manifest.Entries {
		if entry.Platform == runtime.GOOS && entry.Arch == runtime.GOARCH {
			entries = append(entries, entry)
		}
	}
	return entries
}

func manifestDigest(manifest Manifest) string {
	data, _ := json.Marshal(manifest)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func materializeFS(source fs.FS, sourceRoot, destination string, manifest Manifest) error {
	if source == nil {
		return errors.New("embedded resource source is unavailable")
	}
	for _, entry := range manifest.Entries {
		if entry.Platform != runtime.GOOS || entry.Arch != runtime.GOARCH {
			continue
		}
		relative, err := safeRelative(entry.Path)
		if err != nil {
			return err
		}
		inputPath := path.Join(sourceRoot, relative)
		input, err := source.Open(inputPath)
		if err != nil {
			return fmt.Errorf("open packaged resource %s: %w", entry.LogicalName, err)
		}
		data, readErr := fs.ReadFile(source, inputPath)
		_ = input.Close()
		if readErr != nil {
			return readErr
		}
		hash := sha256.Sum256(data)
		if !strings.EqualFold(hex.EncodeToString(hash[:]), entry.SHA256) {
			return fmt.Errorf("resource %s failed packaged hash verification", entry.LogicalName)
		}
		target, err := managerSafeJoin(destination, relative)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("refusing to overwrite staged resource %s", entry.LogicalName)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(entry.Mode))
		if err != nil {
			return err
		}
		if _, err := output.Write(data); err != nil {
			_ = output.Close()
			return err
		}
		if err := output.Sync(); err != nil {
			_ = output.Close()
			return err
		}
		if err := output.Close(); err != nil {
			return err
		}
	}
	return nil
}

func safeRelative(value string) (string, error) {
	value = filepath.ToSlash(value)
	if value == "" || value == "." || strings.HasPrefix(value, "/") || path.IsAbs(value) {
		return "", fmt.Errorf("resource path must be relative: %s", value)
	}
	clean := path.Clean(value)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("resource path escapes root: %s", value)
	}
	return clean, nil
}

func managerSafeJoin(root, relative string) (string, error) {
	if _, err := safeRelative(relative); err != nil {
		return "", err
	}
	joined := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	cleanRoot := filepath.Clean(root)
	if joined != cleanRoot && !strings.HasPrefix(joined, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("resource path escapes root: %s", relative)
	}
	return joined, nil
}

func safeSegment(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}
