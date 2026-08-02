package lumen

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"desktop/internal/platform"
)

const InstallSchemaVersion = 1

type ArtifactManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Version       string `json:"version"`
	Profile       string `json:"profile"`
	Binary        string `json:"binary"`
	SHA256        string `json:"sha256"`
	Signature     string `json:"signature"`
}

type InstallJournal struct {
	SchemaVersion int    `json:"schemaVersion"`
	Phase         string `json:"phase"`
	Version       string `json:"version"`
	Profile       string `json:"profile"`
	TargetHash    string `json:"targetHash"`
}

type CurrentPointer struct {
	SchemaVersion int    `json:"schemaVersion"`
	Version       string `json:"version"`
	Profile       string `json:"profile"`
	Binary        string `json:"binary"`
	SHA256        string `json:"sha256"`
}

type ArtifactFetcher func(context.Context, string) (manifest []byte, artifact []byte, err error)
type ArtifactProbe func(context.Context, string, string, string) error

type PackageInstaller struct {
	Root   string
	Key    ed25519.PublicKey
	Fetch  ArtifactFetcher
	Probe  ArtifactProbe
	Target string
}

func LoadCurrent(root string) (CurrentPointer, error) {
	data, err := os.ReadFile(filepath.Join(root, "current.json"))
	if err != nil {
		return CurrentPointer{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var current CurrentPointer
	if err := decoder.Decode(&current); err != nil {
		return CurrentPointer{}, err
	}
	if current.SchemaVersion != InstallSchemaVersion || strings.TrimSpace(current.Version) == "" || strings.TrimSpace(current.Profile) == "" {
		return CurrentPointer{}, errors.New("invalid Lumen current pointer")
	}
	if err := validateRelativePath(current.Binary); err != nil {
		return CurrentPointer{}, err
	}
	binaryPath, err := safeJoin(root, current.Binary)
	if err != nil {
		return CurrentPointer{}, err
	}
	if err := verifyRegular(binaryPath); err != nil {
		return CurrentPointer{}, err
	}
	return current, nil
}

func (i PackageInstaller) Install(ctx context.Context, profile string) (string, error) {
	if strings.TrimSpace(i.Root) == "" || i.Fetch == nil {
		return "", errors.New("Lumen installer is not configured")
	}
	if i.Probe == nil {
		return "", errors.New("Lumen installer has no authenticated probe")
	}
	manifestBytes, artifact, err := i.Fetch(ctx, profile)
	if err != nil {
		return "", err
	}
	manifest, err := VerifyArtifactManifest(manifestBytes, i.Key)
	if err != nil {
		return "", err
	}
	if manifest.Profile != profile {
		return "", fmt.Errorf("Lumen artifact profile %q does not match %q", manifest.Profile, profile)
	}
	hash := sha256.Sum256(artifact)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), manifest.SHA256) {
		return "", errors.New("Lumen artifact digest does not match its signed manifest")
	}
	if err := validateRelativePath(manifest.Binary); err != nil {
		return "", err
	}
	if err := platform.EnsurePrivateDirectory(i.Root); err != nil {
		return "", err
	}
	versions := filepath.Join(i.Root, "versions")
	if err := platform.EnsurePrivateDirectory(versions); err != nil {
		return "", err
	}
	var tokenBytes [12]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes[:])
	targetName := safeVersionName(manifest.Version) + "-" + safeVersionName(profile)
	target := filepath.Join(versions, targetName)
	staging := filepath.Join(versions, ".staging-"+token)
	if _, err := os.Lstat(target); err == nil {
		return "", errors.New("Lumen target version already exists")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if err := platform.EnsurePrivateDirectory(staging); err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	if err := i.writeJournal(InstallJournal{Phase: "staging", Version: manifest.Version, Profile: profile, TargetHash: manifest.SHA256}); err != nil {
		return "", err
	}
	if err := extractZip(staging, artifact); err != nil {
		return "", err
	}
	binaryPath, err := safeJoin(staging, manifest.Binary)
	if err != nil {
		return "", err
	}
	if err := verifyRegular(binaryPath); err != nil {
		return "", fmt.Errorf("Lumen binary is invalid: %w", err)
	}
	if err := i.writeJournal(InstallJournal{Phase: "verified", Version: manifest.Version, Profile: profile, TargetHash: manifest.SHA256}); err != nil {
		return "", err
	}
	launchToken := token
	if err := i.Probe(ctx, binaryPath, profile, launchToken); err != nil {
		return "", err
	}
	if err := i.writeJournal(InstallJournal{Phase: "promoting", Version: manifest.Version, Profile: profile, TargetHash: manifest.SHA256}); err != nil {
		return "", err
	}
	if err := os.Rename(staging, target); err != nil {
		return "", err
	}
	current := CurrentPointer{SchemaVersion: InstallSchemaVersion, Version: manifest.Version, Profile: profile, Binary: filepath.ToSlash(filepath.Join("versions", targetName, manifest.Binary)), SHA256: manifest.SHA256}
	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return "", err
	}
	if err := platform.WriteAtomic(filepath.Join(i.Root, "current.json"), append(data, '\n'), 0o600); err != nil {
		return "", err
	}
	if err := i.writeJournal(InstallJournal{Phase: "committed", Version: manifest.Version, Profile: profile, TargetHash: manifest.SHA256}); err != nil {
		return "", err
	}
	if err := os.Remove(filepath.Join(i.Root, "install.json")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return manifest.Version, nil
}

func VerifyArtifactManifest(data []byte, key ed25519.PublicKey) (ArtifactManifest, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest ArtifactManifest
	if err := decoder.Decode(&manifest); err != nil {
		return ArtifactManifest{}, err
	}
	if manifest.SchemaVersion != InstallSchemaVersion || strings.TrimSpace(manifest.Version) == "" || strings.TrimSpace(manifest.Profile) == "" {
		return ArtifactManifest{}, errors.New("unsupported or incomplete Lumen artifact manifest")
	}
	if err := validateRelativePath(manifest.Binary); err != nil {
		return ArtifactManifest{}, err
	}
	if len(manifest.SHA256) != sha256.Size*2 {
		return ArtifactManifest{}, errors.New("invalid Lumen artifact SHA-256")
	}
	if _, err := hex.DecodeString(manifest.SHA256); err != nil {
		return ArtifactManifest{}, err
	}
	signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize || len(key) != ed25519.PublicKeySize {
		return ArtifactManifest{}, errors.New("invalid Lumen artifact signature")
	}
	unsigned := manifest
	unsigned.Signature = ""
	payload, _ := json.Marshal(unsigned)
	if !ed25519.Verify(key, payload, signature) {
		return ArtifactManifest{}, errors.New("invalid Lumen artifact signature")
	}
	return manifest, nil
}

func (i PackageInstaller) writeJournal(journal InstallJournal) error {
	journal.SchemaVersion = InstallSchemaVersion
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteAtomic(filepath.Join(i.Root, "install.json"), append(data, '\n'), 0o600)
}

func extractZip(root string, data []byte) error {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, item := range archive.File {
		if err := validateRelativePath(item.Name); err != nil {
			return err
		}
		target, err := safeJoin(root, item.Name)
		if err != nil {
			return err
		}
		if item.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			continue
		}
		if item.Mode()&os.ModeSymlink != 0 || !item.Mode().IsRegular() {
			return fmt.Errorf("Lumen archive contains a non-regular entry %q", item.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		input, err := item.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		_ = input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func verifyRegular(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errors.New("path is not a regular file")
	}
	return nil
}

func validateRelativePath(value string) error {
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) {
		return fmt.Errorf("unsafe Lumen archive path %q", value)
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("unsafe Lumen archive path %q", value)
	}
	return nil
}

func safeJoin(root, relative string) (string, error) {
	if err := validateRelativePath(relative); err != nil {
		return "", err
	}
	root = filepath.Clean(root)
	joined := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	if joined != root && !strings.HasPrefix(joined, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes Lumen root: %q", relative)
	}
	return joined, nil
}

func safeVersionName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	return builder.String()
}
