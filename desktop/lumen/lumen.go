// Package lumen installs and supervises a local Lumen Hub — the optional ML
// inference server — the same way the supervisor package owns the private
// PostgreSQL: download once into the app-data dir, then run it as a child
// process for the lifetime of the app.
//
// This launcher deliberately lives in the desktop host (not Lumen-SDK): the
// SDK is the consumer-side library (discovery + inference), while installing
// binaries is a deployment concern with exactly one consumer — this app.
// The contract with Lumen-Hub releases is small: a manifest.json at the
// latest-release URL listing one zip per build profile, sha256-pinned.
package lumen

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// DefaultManifestURL always points at the newest release; the app never
	// needs updating to install a newer hub.
	DefaultManifestURL = "https://github.com/EdwinZhanCN/Lumen-Hub/releases/latest/download/manifest.json"

	// GRPCEndpoint is where the supervised hub listens. The desktop server
	// config pins it as a Lumen static node unconditionally: the SDK treats
	// static entries as address facts (never expired, reconnect-managed), so
	// the pin is harmless while the hub is not installed or not running.
	GRPCEndpoint = "127.0.0.1:50051"
)

// Manifest is the release metadata `cargo xtask release-metadata` publishes.
type Manifest struct {
	Version string     `json:"version"`
	Hub     []Artifact `json:"hub"`
}

// Artifact is one per-profile hub build inside a release.
type Artifact struct {
	Profile  string `json:"profile"`
	FileName string `json:"file_name"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
}

// ErrUnsupportedHost means no hub build profile exists for this OS/arch.
var ErrUnsupportedHost = errors.New("no Lumen Hub build for this platform")

// ProfileForHost maps the host platform to the release profile the desktop
// installs. Apple Silicon gets the Metal build; Windows gets the portable
// wgpu build (DX12/Vulkan picked at runtime, works on any WDDM GPU).
func ProfileForHost() (string, error) {
	return profileFor(runtime.GOOS, runtime.GOARCH)
}

func profileFor(goos, goarch string) (string, error) {
	choices, err := BackendChoicesFor(goos, goarch)
	if err != nil {
		return "", err
	}
	return choices[0].Profile, nil
}

// FetchManifest downloads and parses the release manifest.
func FetchManifest(ctx context.Context, url string) (Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Manifest{}, err
	}
	req.Header.Set("User-Agent", "Lumilio-Photos-Desktop")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Manifest{}, fmt.Errorf("fetch Lumen manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Manifest{}, fmt.Errorf("fetch Lumen manifest: HTTP %d", resp.StatusCode)
	}
	var m Manifest
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("parse Lumen manifest: %w", err)
	}
	return m, nil
}

// InstallState records what is installed under the lumen dir, persisted as
// installed.json so upgrades can compare against the manifest version.
type InstallState struct {
	Version string `json:"version"`
	Profile string `json:"profile"`
}

// hubDir is where the unpacked build lives (bin/ + warmup/ + licenses).
func hubDir(dir string) string { return filepath.Join(dir, "hub") }

func stateFile(dir string) string { return filepath.Join(dir, "installed.json") }

func previousStateFile(dir string) string { return filepath.Join(dir, "installed.previous.json") }

// HubBinary returns the path of the installed hub executable.
func HubBinary(dir string) string {
	name := "lumen-hub"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(hubDir(dir), "bin", name)
}

// Installed reports the persisted install state, if a usable install exists.
func Installed(dir string) (InstallState, bool) {
	data, err := os.ReadFile(stateFile(dir))
	if err != nil {
		return InstallState{}, false
	}
	var s InstallState
	if err := json.Unmarshal(data, &s); err != nil {
		return InstallState{}, false
	}
	if _, err := os.Stat(HubBinary(dir)); err != nil {
		return InstallState{}, false
	}
	return s, true
}

// Install downloads the hub build for this host into dir (replacing any
// previous install), verifying the manifest's sha256. logf receives progress
// lines. It does not start anything.
func Install(ctx context.Context, dir, manifestURL string, logf func(string, ...any)) (InstallState, error) {
	profile, err := ProfileForHost()
	if err != nil {
		return InstallState{}, err
	}
	return InstallProfile(ctx, dir, manifestURL, profile, logf)
}

// InstallProfile installs the selected launcher-compatible release profile.
func InstallProfile(ctx context.Context, dir, manifestURL, profile string, logf func(string, ...any)) (InstallState, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	choices, err := BackendChoicesForHost()
	if err != nil {
		return InstallState{}, err
	}
	allowed := false
	for _, choice := range choices {
		if choice.Profile == profile {
			allowed = true
		}
	}
	if !allowed {
		return InstallState{}, fmt.Errorf("%w: profile %q", ErrUnsupportedHost, profile)
	}
	if manifestURL == "" {
		manifestURL = DefaultManifestURL
	}

	logf("lumen: fetching manifest %s", manifestURL)
	manifest, err := FetchManifest(ctx, manifestURL)
	if err != nil {
		return InstallState{}, err
	}
	var artifact *Artifact
	for i := range manifest.Hub {
		if manifest.Hub[i].Profile == profile {
			artifact = &manifest.Hub[i]
			break
		}
	}
	if artifact == nil {
		return InstallState{}, fmt.Errorf("release %s has no hub build for profile %q", manifest.Version, profile)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return InstallState{}, err
	}
	zipPath := filepath.Join(dir, artifact.FileName+".part")
	defer os.Remove(zipPath)
	logf("lumen: downloading %s (%s)", artifact.FileName, manifest.Version)
	if err := downloadVerified(ctx, artifact.URL, artifact.SHA256, zipPath); err != nil {
		return InstallState{}, err
	}

	// Extract into a scratch dir, then swap it in, so a failed/cancelled
	// install never leaves a half-written hub/ behind.
	next := hubDir(dir) + ".next"
	if err := os.RemoveAll(next); err != nil {
		return InstallState{}, err
	}
	logf("lumen: unpacking %s", artifact.FileName)
	if err := extractZip(zipPath, next); err != nil {
		return InstallState{}, err
	}
	if _, err := os.Stat(filepath.Join(next, "bin")); err != nil {
		return InstallState{}, fmt.Errorf("hub archive %s has no bin/ directory", artifact.FileName)
	}
	previous := hubDir(dir) + ".previous"
	if err := os.RemoveAll(previous); err != nil {
		return InstallState{}, err
	}
	if _, err := os.Stat(hubDir(dir)); err == nil {
		if oldState, readErr := os.ReadFile(stateFile(dir)); readErr == nil {
			_ = os.WriteFile(previousStateFile(dir), oldState, 0o600)
		}
		if err := os.Rename(hubDir(dir), previous); err != nil {
			return InstallState{}, err
		}
	}
	if err := os.Rename(next, hubDir(dir)); err != nil {
		_ = os.Rename(previous, hubDir(dir))
		return InstallState{}, err
	}

	state := InstallState{Version: manifest.Version, Profile: profile}
	data, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(stateFile(dir), data, 0o600); err != nil {
		return InstallState{}, err
	}
	logf("lumen: installed %s (%s)", state.Version, state.Profile)
	return state, nil
}

// RestorePrevious rolls back the most recent successful install swap. It is
// used when the new binary cannot start or become ready.
func RestorePrevious(dir string) bool {
	previous := hubDir(dir) + ".previous"
	if _, err := os.Stat(previous); err != nil {
		return false
	}
	failed := hubDir(dir) + ".failed"
	_ = os.RemoveAll(failed)
	if err := os.Rename(hubDir(dir), failed); err != nil {
		return false
	}
	if err := os.Rename(previous, hubDir(dir)); err != nil {
		_ = os.Rename(failed, hubDir(dir))
		return false
	}
	if data, err := os.ReadFile(previousStateFile(dir)); err == nil {
		_ = os.WriteFile(stateFile(dir), data, 0o600)
	}
	_ = os.RemoveAll(failed)
	return true
}

// downloadVerified streams url to path, failing unless the sha256 matches.
func downloadVerified(ctx context.Context, url, wantSHA, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Lumilio-Photos-Desktop")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(f, hasher), resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return fmt.Errorf("download %s: %w", url, copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if got := hex.EncodeToString(hasher.Sum(nil)); !strings.EqualFold(got, wantSHA) {
		return fmt.Errorf("checksum mismatch for %s: want %s, got %s", url, wantSHA, got)
	}
	return nil
}

// extractZip unpacks a hub release zip into dest, stripping the single
// top-level directory the archives wrap everything in
// (lumen-hub-<profile>/...). The zips carry no unix permission bits, so every
// file under bin/ is made executable.
func extractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", zipPath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		rel := stripFirstComponent(f.Name)
		if rel == "" {
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		// Zip-slip guard: the target must stay inside dest.
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry escapes destination: %q", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasPrefix(rel, "bin/") {
			mode = 0o755
		}
		if err := writeZipFile(f, target, mode); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(f *zip.File, target string, mode os.FileMode) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

// stripFirstComponent drops the archive's wrapping directory:
// "lumen-hub-x/bin/lumen-hub" → "bin/lumen-hub"; the wrapper itself → "".
func stripFirstComponent(name string) string {
	name = strings.TrimPrefix(name, "./")
	if i := strings.IndexByte(name, '/'); i >= 0 {
		return name[i+1:]
	}
	return ""
}
