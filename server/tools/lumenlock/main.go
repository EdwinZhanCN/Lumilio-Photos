// Command lumenlock owns the Lumen Hub release pin and the Desktop's generated
// artifact/preset catalog.
//
//   - check  validates only committed local state. It is deterministic and safe
//     for every CI run.
//   - verify proves the pin against the remote release without writing.
//   - sync   performs the same remote proof, then atomically reconciles derived
//     lock fields and generated files.
//
// Renovate owns only lock.release. All other lock fields are derived by sync.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	lockFileName     = "lumen.lock.json"
	catalogPath      = "desktop/internal/lumen/release_catalog.go"
	controlProtoPath = "desktop/internal/lumen/controlv1/control.proto"
	userAgent        = "Lumilio-Photos-lumenlock"
	legacyChecksName = "checksums.txt"
	lockSchema       = 2
)

var desktopProfiles = []string{
	"darwin-arm64-metal",
	"darwin-arm64-cpu",
	"windows-x64-cpu",
	"windows-x64-gpu",
}

var (
	releaseRe    = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)
	repoRe       = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(\.git)?$`)
	sha256Re     = regexp.MustCompile(`^[a-f0-9]{64}$`)
	sha40Re      = regexp.MustCompile(`^[a-f0-9]{40}$`)
	presetEnvRe  = regexp.MustCompile(`LUMEN_PRESET:\s*([a-z][a-z0-9-]*)`)
	composeFiles = []string{
		"deploy/compose/lumen-cpu.compose.yml",
		"deploy/compose/lumen-vulkan.compose.yml",
		"deploy/compose/lumen-cuda.compose.yml",
	}
)

type lock struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Repository         string `json:"repository"`
	Release            string `json:"release"`
	Revision           string `json:"revision"`
	ManifestSHA256     string `json:"manifestSha256"`
	CatalogSHA256      string `json:"catalogSha256"`
	ControlProtoSHA256 string `json:"controlProtoSha256"`
}

type manifest struct {
	SchemaVersion *int   `json:"schemaVersion"`
	Version       string `json:"version"`
	Hub           []struct {
		Profile  string `json:"profile"`
		FileName string `json:"file_name"`
		URL      string `json:"url"`
		SHA256   string `json:"sha256"`
	} `json:"hub"`
	Presets []struct {
		ID string `json:"id"`
	} `json:"presets"`
}

type artifact struct {
	profile  string
	fileName string
	sha256   string
}

type remoteState struct {
	revision     string
	manifestHash string
	catalog      []byte
	protoHash    string
}

func main() {
	if len(os.Args) < 3 || (os.Args[1] != "sync" && os.Args[1] != "verify" && os.Args[1] != "check") {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/lumenlock <check|verify|sync> <repository-root> [--release vX.Y.Z]")
		os.Exit(2)
	}
	root, err := filepath.Abs(os.Args[2])
	if err != nil {
		fail(err)
	}
	target, err := parseReleaseArg(os.Args[3:])
	if err != nil {
		fail(err)
	}
	if target != "" && os.Args[1] != "sync" {
		fail(errors.New("--release is only valid with sync"))
	}
	if err := run(os.Args[1], root, target); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lumenlock:", err)
	os.Exit(1)
}

func run(mode, root, targetRelease string) error {
	lockPath := filepath.Join(root, lockFileName)
	current, err := readLock(lockPath)
	if err != nil {
		return err
	}
	if mode == "check" {
		return checkLocal(root, current)
	}
	if targetRelease != "" {
		current.Release = targetRelease
	}

	remote, err := inspectRemote(root, current)
	if err != nil {
		return err
	}
	if mode == "verify" {
		if err := compareRemote(root, current, remote); err != nil {
			return err
		}
		fmt.Printf("lumen:verify ok — %s release, checksums, provenance, proto, presets, and Desktop profiles verified\n", current.Release)
		return nil
	}

	catalogAbs := filepath.Join(root, filepath.FromSlash(catalogPath))
	changed := false
	if existing, _ := os.ReadFile(catalogAbs); !bytesEqual(existing, remote.catalog) {
		if err := writeAtomic(catalogAbs, remote.catalog, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", catalogPath, err)
		}
		changed = true
		fmt.Printf("lumen:sync regenerated %s from %s\n", catalogPath, current.Release)
	}

	current.Revision = remote.revision
	current.ManifestSHA256 = remote.manifestHash
	current.CatalogSHA256 = sha256Hex(remote.catalog)
	current.ControlProtoSHA256 = remote.protoHash
	encoded, err := encodeLock(current)
	if err != nil {
		return err
	}
	existingLock, _ := os.ReadFile(lockPath)
	if !bytesEqual(existingLock, encoded) {
		if err := writeAtomic(lockPath, encoded, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", lockFileName, err)
		}
		changed = true
		fmt.Printf("lumen:sync reconciled %s (revision %s)\n", lockFileName, current.Revision[:12])
	}
	if !changed {
		fmt.Printf("lumen:sync ok — %s and %s are already in sync\n", lockFileName, catalogPath)
	}
	return checkLocal(root, current)
}

func parseReleaseArg(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) != 2 || args[0] != "--release" {
		return "", errors.New("expected --release vX.Y.Z")
	}
	if !releaseRe.MatchString(args[1]) {
		return "", fmt.Errorf("--release must be a semver tag like v0.2.0")
	}
	return args[1], nil
}

func readLock(path string) (lock, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return lock{}, fmt.Errorf("read %s: %w", path, err)
	}
	return parseLock(raw)
}

func checkLocal(root string, current lock) error {
	catalog, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(catalogPath)))
	if err != nil {
		return fmt.Errorf("read %s: %w", catalogPath, err)
	}
	if got := sha256Hex(catalog); got != current.CatalogSHA256 {
		return fmt.Errorf("%s hash is %s, lock expects %s; run `task lumen:sync`", catalogPath, got, current.CatalogSHA256)
	}
	if !strings.Contains(string(catalog), fmt.Sprintf("const OfficialReleaseVersion = %q", current.Release)) {
		return fmt.Errorf("%s does not declare lock release %s", catalogPath, current.Release)
	}

	proto, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(controlProtoPath)))
	if err != nil {
		return fmt.Errorf("read %s: %w", controlProtoPath, err)
	}
	if got := sha256Hex(proto); got != current.ControlProtoSHA256 {
		return fmt.Errorf("%s hash is %s, lock expects %s; re-vendor the proto and run `task lumen:sync`", controlProtoPath, got, current.ControlProtoSHA256)
	}

	composePresets, err := composePresetNames(root)
	if err != nil {
		return err
	}
	for _, preset := range composePresets {
		if !strings.Contains(string(catalog), fmt.Sprintf("\t%q,", preset)) {
			return fmt.Errorf("Compose references preset %q, but %s does not export it", preset, catalogPath)
		}
	}
	fmt.Printf("lumen:check ok — committed lock, catalog, proto, and Compose intent are internally consistent for %s\n", current.Release)
	return nil
}

func inspectRemote(root string, current lock) (remoteState, error) {
	baseURL := strings.TrimSuffix(current.Repository, ".git")
	manifestURL := fmt.Sprintf("%s/releases/download/%s/manifest.json", baseURL, current.Release)
	manifestRaw, err := fetch(manifestURL)
	if err != nil {
		return remoteState{}, err
	}
	parsed, err := parseManifest(manifestRaw)
	if err != nil {
		return remoteState{}, err
	}
	if parsed.Version != current.Release {
		return remoteState{}, fmt.Errorf("manifest version %q does not match lock release %q", parsed.Version, current.Release)
	}

	checks, err := fetchChecksums(baseURL, current.Release)
	if err != nil {
		return remoteState{}, err
	}
	manifestHash := sha256Hex(manifestRaw)
	if err := verifyManifestChecksum(checks, manifestHash); err != nil {
		return remoteState{}, err
	}

	byProfile := make(map[string]artifact, len(parsed.Hub))
	for _, entry := range parsed.Hub {
		want := checks[entry.FileName]
		if want == "" {
			return remoteState{}, fmt.Errorf("SHA256SUMS is missing artifact %s", entry.FileName)
		}
		if !strings.EqualFold(want, entry.SHA256) {
			return remoteState{}, fmt.Errorf("artifact %s digest does not match SHA256SUMS", entry.FileName)
		}
		if _, duplicate := byProfile[entry.Profile]; duplicate {
			return remoteState{}, fmt.Errorf("release manifest repeats profile %s", entry.Profile)
		}
		byProfile[entry.Profile] = artifact{profile: entry.Profile, fileName: entry.FileName, sha256: entry.SHA256}
	}
	for _, profile := range desktopProfiles {
		if _, ok := byProfile[profile]; !ok {
			return remoteState{}, fmt.Errorf("release %s lacks Desktop profile %s", current.Release, profile)
		}
	}

	presets, err := manifestPresetNames(parsed)
	if err != nil {
		return remoteState{}, err
	}
	composePresets, err := composePresetNames(root)
	if err != nil {
		return remoteState{}, err
	}
	known := make(map[string]bool, len(presets))
	for _, preset := range presets {
		known[preset] = true
	}
	for _, preset := range composePresets {
		if !known[preset] {
			return remoteState{}, fmt.Errorf("deploy/compose references LUMEN_PRESET=%s which release %s does not define", preset, current.Release)
		}
	}

	revision, err := resolveTagCommit(current.Repository, current.Release)
	if err != nil {
		return remoteState{}, err
	}
	protoHash, err := verifyVendoredControlProto(root, current.Repository, revision)
	if err != nil {
		return remoteState{}, err
	}
	catalog, err := generateCatalog(current.Release, desktopProfiles, byProfile, presets)
	if err != nil {
		return remoteState{}, err
	}
	return remoteState{
		revision: revision, manifestHash: manifestHash, catalog: []byte(catalog), protoHash: protoHash,
	}, nil
}

func compareRemote(root string, current lock, remote remoteState) error {
	if current.Revision != remote.revision || current.ManifestSHA256 != remote.manifestHash {
		return fmt.Errorf("%s remote provenance is stale; run `task lumen:sync`", lockFileName)
	}
	catalog, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(catalogPath)))
	if err != nil {
		return err
	}
	if !bytesEqual(catalog, remote.catalog) || current.CatalogSHA256 != sha256Hex(remote.catalog) {
		return fmt.Errorf("%s is stale for %s; run `task lumen:sync`", catalogPath, current.Release)
	}
	if current.ControlProtoSHA256 != remote.protoHash {
		return fmt.Errorf("%s hash in %s is stale; run `task lumen:sync`", controlProtoPath, lockFileName)
	}
	return checkLocal(root, current)
}

func parseLock(raw []byte) (lock, error) {
	var parsed lock
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return parsed, fmt.Errorf("decode %s: %w", lockFileName, err)
	}
	if parsed.SchemaVersion != lockSchema {
		return parsed, fmt.Errorf("%s must use schemaVersion %d", lockFileName, lockSchema)
	}
	if !repoRe.MatchString(parsed.Repository) {
		return parsed, fmt.Errorf("%s repository must be an HTTPS GitHub URL", lockFileName)
	}
	if !releaseRe.MatchString(parsed.Release) {
		return parsed, fmt.Errorf("%s release must be a semver tag like v0.1.1", lockFileName)
	}
	if !sha40Re.MatchString(parsed.Revision) {
		return parsed, fmt.Errorf("%s revision must be a full 40-character commit SHA", lockFileName)
	}
	for name, value := range map[string]string{
		"manifestSha256":     parsed.ManifestSHA256,
		"catalogSha256":      parsed.CatalogSHA256,
		"controlProtoSha256": parsed.ControlProtoSHA256,
	} {
		if !sha256Re.MatchString(value) {
			return parsed, fmt.Errorf("%s %s must be a lowercase SHA-256", lockFileName, name)
		}
	}
	return parsed, nil
}

func parseManifest(raw []byte) (manifest, error) {
	var parsed manifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return parsed, fmt.Errorf("decode release manifest: %w", err)
	}
	if parsed.SchemaVersion == nil || *parsed.SchemaVersion < 2 {
		return parsed, fmt.Errorf("release manifest must use schemaVersion 2 or newer; publish a Hub release with managed config support")
	}
	if *parsed.SchemaVersion > 2 {
		return parsed, fmt.Errorf("release manifest schemaVersion %d is newer than supported", *parsed.SchemaVersion)
	}
	if !releaseRe.MatchString(parsed.Version) {
		return parsed, fmt.Errorf("release manifest has an invalid version %q", parsed.Version)
	}
	if len(parsed.Hub) == 0 {
		return parsed, errors.New("release manifest has no hub artifacts")
	}
	for _, entry := range parsed.Hub {
		if entry.Profile == "" || entry.FileName == "" || entry.URL == "" || !sha256Re.MatchString(entry.SHA256) {
			return parsed, errors.New("release manifest has an invalid artifact entry")
		}
	}
	return parsed, nil
}

func manifestPresetNames(parsed manifest) ([]string, error) {
	if len(parsed.Presets) == 0 {
		return nil, errors.New("release manifest has no presets; Desktop managed config requires the canonical preset catalog")
	}
	seen := make(map[string]bool, len(parsed.Presets))
	presets := make([]string, 0, len(parsed.Presets))
	for _, preset := range parsed.Presets {
		if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(preset.ID) {
			return nil, fmt.Errorf("release manifest has invalid preset id %q", preset.ID)
		}
		if seen[preset.ID] {
			return nil, fmt.Errorf("release manifest repeats preset %q", preset.ID)
		}
		seen[preset.ID] = true
		presets = append(presets, preset.ID)
	}
	return presets, nil
}

func verifyManifestChecksum(checks map[string]string, manifestHash string) error {
	want, ok := checks["manifest.json"]
	if !ok || want == "" {
		return errors.New("SHA256SUMS is missing manifest.json")
	}
	if !strings.EqualFold(want, manifestHash) {
		return errors.New("SHA256SUMS digest for manifest.json does not match the downloaded manifest")
	}
	return nil
}

func fetchChecksums(baseURL, release string) (map[string]string, error) {
	var lastErr error
	for _, name := range []string{"SHA256SUMS", legacyChecksName} {
		raw, err := fetch(fmt.Sprintf("%s/releases/download/%s/%s", baseURL, release, name))
		if err != nil {
			lastErr = err
			continue
		}
		checks := make(map[string]string)
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 || !sha256Re.MatchString(fields[0]) {
				return nil, fmt.Errorf("%s contains an invalid line: %q", name, line)
			}
			checks[strings.TrimPrefix(fields[1], "*")] = fields[0]
		}
		return checks, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no checksum file found")
	}
	return nil, fmt.Errorf("fetch SHA256SUMS: %w", lastErr)
}

func resolveTagCommit(repository, release string) (string, error) {
	output, err := exec.Command("git", "ls-remote", "--tags", repository, "refs/tags/"+release, "refs/tags/"+release+"^{}").Output()
	if err != nil {
		return "", fmt.Errorf("resolve tag %s: %w", release, err)
	}
	for _, suffix := range []string{"^{}", ""} {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && strings.HasSuffix(fields[1], suffix) && (suffix != "" || !strings.HasSuffix(fields[1], "^{}")) {
				return fields[0], nil
			}
		}
	}
	return "", fmt.Errorf("git ls-remote found no ref for tag %s", release)
}

func composePresetNames(root string) ([]string, error) {
	var names []string
	for _, relative := range composeFiles {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", relative, err)
		}
		for _, match := range presetEnvRe.FindAllStringSubmatch(string(raw), -1) {
			names = append(names, match[1])
		}
	}
	return names, nil
}

func verifyVendoredControlProto(root, repository, revision string) (string, error) {
	path := filepath.Join(root, filepath.FromSlash(controlProtoPath))
	vendored, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", controlProtoPath, err)
	}
	rawURL, err := controlProtoRawURL(repository, revision)
	if err != nil {
		return "", err
	}
	authoritative, err := fetch(rawURL)
	if err != nil {
		return "", err
	}
	if !bytesEqual(authoritative, vendored) {
		return "", fmt.Errorf("%s drifted from control.proto at revision %s; re-vendor it and run `task desktop:proto:gen`", controlProtoPath, revision[:12])
	}
	return sha256Hex(vendored), nil
}

func controlProtoRawURL(repository, revision string) (string, error) {
	match := repoRe.FindStringSubmatch(repository)
	if match == nil {
		return "", fmt.Errorf("unexpected repository %q", repository)
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/crates/lumen-hub/proto/control.proto", match[1], match[2], revision), nil
}

func generateCatalog(release string, profiles []string, byProfile map[string]artifact, presets []string) (string, error) {
	sortedProfiles := append([]string(nil), profiles...)
	sort.Strings(sortedProfiles)
	var builder strings.Builder
	fmt.Fprintf(&builder, "// Code generated by `task lumen:sync` from the Lumen-Hub release catalog.\n")
	fmt.Fprintf(&builder, "// Source: release %s. DO NOT EDIT.\n\n", release)
	builder.WriteString("package lumen\n\n")
	fmt.Fprintf(&builder, "// OfficialReleaseVersion is pinned into the Desktop binary.\nconst OfficialReleaseVersion = %q\n\n", release)
	builder.WriteString("// OfficialSetupPresets is the only preset allow-list consumed by Desktop.\nvar OfficialSetupPresets = []string{\n")
	for _, preset := range presets {
		fmt.Fprintf(&builder, "\t%q,\n", preset)
	}
	builder.WriteString("}\n\n")
	builder.WriteString("var officialReleaseArtifacts = map[string]ReleaseArtifact{\n")
	for _, profile := range sortedProfiles {
		entry, ok := byProfile[profile]
		if !ok {
			return "", fmt.Errorf("desktop profile %s missing from release catalog", profile)
		}
		binary := "bin/lumen-hub"
		if strings.Contains(profile, "windows") {
			binary = "bin/lumen-hub.exe"
		}
		fmt.Fprintf(&builder, "\t%q: {\n", profile)
		fmt.Fprintf(&builder, "\t\tVersion: OfficialReleaseVersion, Profile: %q,\n", profile)
		fmt.Fprintf(&builder, "\t\tFileName: %q,\n", entry.fileName)
		fmt.Fprintf(&builder, "\t\tURL:      officialReleasePrefix + OfficialReleaseVersion + \"/%s\",\n", entry.fileName)
		fmt.Fprintf(&builder, "\t\tSHA256:   %q,\n", entry.sha256)
		fmt.Fprintf(&builder, "\t\tBinary:   %q,\n\t},\n", binary)
	}
	builder.WriteString("}\n")
	return builder.String(), nil
}

func encodeLock(value lock) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".lumenlock-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func fetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment, ForceAttemptHTTP2: true, ResponseHeaderTimeout: 30 * time.Second,
	}}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", userAgent)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
