package lumen

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseInstallerInstallsPinnedUpstreamLayout(t *testing.T) {
	profile := "darwin-arm64-metal"
	artifactBytes := releaseZip(t, "lumen-hub-"+profile, "bin/lumen-hub", []byte("hub"))
	digest := sha256.Sum256(artifactBytes)
	artifact := ReleaseArtifact{
		Version: OfficialReleaseVersion, Profile: profile,
		FileName: "lumen-hub-" + profile + ".zip",
		URL:      officialReleasePrefix + OfficialReleaseVersion + "/lumen-hub-" + profile + ".zip",
		SHA256:   hex.EncodeToString(digest[:]), Binary: "bin/lumen-hub",
	}
	root := filepath.Join(t.TempDir(), "lumen")
	probed := ""
	installer := &ReleaseInstaller{
		Root: root, Artifacts: map[string]ReleaseArtifact{profile: artifact},
		Download: func(_ context.Context, _ ReleaseArtifact, target string) error {
			return os.WriteFile(target, artifactBytes, 0o600)
		},
		Probe: func(_ context.Context, binary string) error { probed = binary; return nil },
	}
	version, err := installer.Install(context.Background(), profile)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if version != OfficialReleaseVersion {
		t.Fatalf("version = %q", version)
	}
	current, err := LoadCurrent(root)
	if err != nil {
		t.Fatalf("load current: %v", err)
	}
	if current.Profile != profile || probed == "" {
		t.Fatalf("current = %#v, probe = %q", current, probed)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(current.Binary))); err != nil {
		t.Fatalf("installed binary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "install.json")); !os.IsNotExist(err) {
		t.Fatalf("install journal remains: %v", err)
	}
}

func TestReleaseInstallerRejectsDigestMismatch(t *testing.T) {
	profile := "darwin-arm64-metal"
	artifact := officialReleaseArtifacts[profile]
	installer := &ReleaseInstaller{
		Root:      filepath.Join(t.TempDir(), "lumen"),
		Artifacts: map[string]ReleaseArtifact{profile: artifact},
		Download: func(_ context.Context, _ ReleaseArtifact, target string) error {
			return os.WriteFile(target, []byte("tampered"), 0o600)
		},
		Probe: func(context.Context, string) error { t.Fatal("probe called for tampered artifact"); return nil },
	}
	if _, err := installer.Install(context.Background(), profile); err == nil {
		t.Fatal("tampered release artifact was accepted")
	}
}

func TestReconcileOfficialReleaseCompletesPromotedTarget(t *testing.T) {
	profile := "darwin-arm64-metal"
	artifactBytes := releaseZip(t, "lumen-hub-"+profile, "bin/lumen-hub", []byte("hub"))
	digest := sha256.Sum256(artifactBytes)
	artifact := officialReleaseArtifacts[profile]
	artifact.SHA256 = hex.EncodeToString(digest[:])
	root := filepath.Join(t.TempDir(), "lumen")
	installer := &ReleaseInstaller{
		Root: root, Artifacts: map[string]ReleaseArtifact{profile: artifact},
		Download: func(_ context.Context, _ ReleaseArtifact, target string) error {
			return os.WriteFile(target, artifactBytes, 0o600)
		},
		Probe: func(context.Context, string) error { return nil },
	}
	// Reconciliation trusts the embedded catalog, so temporarily install the
	// test digest there as well and restore it before the test returns.
	original := officialReleaseArtifacts[profile]
	officialReleaseArtifacts[profile] = artifact
	t.Cleanup(func() { officialReleaseArtifacts[profile] = original })
	if _, err := installer.Install(context.Background(), profile); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "current.json")); err != nil {
		t.Fatal(err)
	}
	target := safeVersionName(artifact.Version) + "-" + safeVersionName(profile)
	journal := InstallJournal{
		Phase: "promoting", Version: artifact.Version, Profile: profile,
		TargetHash: artifact.SHA256, Target: target,
	}
	if err := (PackageInstaller{Root: root}).writeJournal(journal); err != nil {
		t.Fatal(err)
	}
	if err := ReconcileOfficialReleaseInstall(root); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if _, err := LoadCurrent(root); err != nil {
		t.Fatalf("load reconciled current: %v", err)
	}
}

func TestDefaultReleaseProfiles(t *testing.T) {
	if profile, ok := DefaultReleaseProfile("darwin", "arm64"); !ok || profile != "darwin-arm64-metal" {
		t.Fatalf("darwin profile = %q, %v", profile, ok)
	}
	if profile, ok := DefaultReleaseProfile("windows", "amd64"); !ok || profile != "windows-x64-cpu" {
		t.Fatalf("windows profile = %q, %v", profile, ok)
	}
	if _, ok := DefaultReleaseProfile("darwin", "amd64"); ok {
		t.Fatal("unsupported Darwin x64 profile was accepted")
	}
}

func TestProbeLumenBinaryValidatesExecutableHeaderWithoutRunningIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lumen-hub")
	if err := os.WriteFile(path, []byte{0xcf, 0xfa, 0xed, 0xfe, 0, 0, 0, 0}, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := probeLumenBinary(context.Background(), path); err != nil {
		t.Fatalf("probe Mach-O: %v", err)
	}
	if err := os.WriteFile(path, []byte("not an executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := probeLumenBinary(context.Background(), path); err == nil {
		t.Fatal("invalid executable header was accepted")
	}
}

func releaseZip(t *testing.T, root, name string, data []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	item, err := archive.Create(root + "/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := item.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
