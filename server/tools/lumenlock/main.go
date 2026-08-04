// Command lumenlock owns the Lumen Hub release pin (lumen.lock.json) and the
// Desktop's embedded release catalog.
//
//   - `sync`  reconciles the lock's derived fields (revision, manifestSha256)
//     and regenerates desktop/internal/lumen/release_catalog.go from the
//     pinned release's manifest.json, after verifying SHA256SUMS and the
//     consumer builds (Desktop profiles, Compose presets).
//   - `check` performs the same verification without writing anything; it is
//     the CI gate that fails when the pin was bumped but the catalog or the
//     lock's derived fields are stale.
//
// The only Renovate-managed field is `release` (github-releases datasource).
// Everything else is derived data produced by `sync`.

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
	userAgent        = "Lumilio-Photos-lumenlock"
	legacyChecksName = "checksums.txt"
)

// Desktop release profiles: decision 5 pins these four exactly.
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
	SchemaVersion  int    `json:"schemaVersion"`
	Repository     string `json:"repository"`
	Release        string `json:"release"`
	Revision       string `json:"revision"`
	ManifestSHA256 string `json:"manifestSha256"`
}

type manifest struct {
	SchemaVersion *int   `json:"schemaVersion"`
	Version       string `json:"version"`
	Hub           []struct {
		Profile  string `json:"profile"`
		FileName string `json:"file_name"`
		URL      string `json:"url"`
		Sha256   string `json:"sha256"`
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

func main() {
	if len(os.Args) != 3 || (os.Args[1] != "sync" && os.Args[1] != "check") {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/lumenlock <sync|check> <repository-root>")
		os.Exit(2)
	}
	mode := os.Args[1]
	root, err := filepath.Abs(os.Args[2])
	if err != nil {
		fail(err)
	}
	if err := run(mode, root); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "lumenlock:", err)
	os.Exit(1)
}

func run(mode, root string) error {
	lockPath := filepath.Join(root, lockFileName)
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", lockPath, err)
	}
	lock, err := parseLock(raw)
	if err != nil {
		return err
	}

	baseURL := strings.TrimSuffix(lock.Repository, ".git")
	manifestURL := fmt.Sprintf("%s/releases/download/%s/manifest.json", baseURL, lock.Release)
	manifestRaw, err := fetch(manifestURL)
	if err != nil {
		return err
	}
	manifest, err := parseManifest(manifestRaw)
	if err != nil {
		return err
	}
	if manifest.Version != lock.Release {
		return fmt.Errorf("manifest version %q does not match lock release %q", manifest.Version, lock.Release)
	}

	// Catalog integrity: SHA256SUMS must cover the manifest itself and every
	// artifact with a matching digest. Releases before the SHA256SUMS rename
	// published checksums.txt; both names are accepted, SHA256SUMS first.
	checks, err := fetchChecksums(baseURL, lock.Release)
	if err != nil {
		return err
	}
	manifestHash := sha256Hex(manifestRaw)
	if want := checks["manifest.json"]; want != "" && !strings.EqualFold(want, manifestHash) {
		return fmt.Errorf("SHA256SUMS digest for manifest.json does not match the downloaded manifest")
	}
	artifacts := make([]artifact, 0, len(manifest.Hub))
	for _, entry := range manifest.Hub {
		if !sha256Re.MatchString(entry.Sha256) {
			return fmt.Errorf("artifact %s has an invalid SHA-256", entry.Profile)
		}
		want := checks[entry.FileName]
		if want == "" {
			return fmt.Errorf("SHA256SUMS is missing artifact %s", entry.FileName)
		}
		if !strings.EqualFold(want, entry.Sha256) {
			return fmt.Errorf("artifact %s digest does not match SHA256SUMS", entry.FileName)
		}
		artifacts = append(artifacts, artifact{profile: entry.Profile, fileName: entry.FileName, sha256: entry.Sha256})
	}
	if len(artifacts) == 0 {
		return fmt.Errorf("release %s has no hub artifacts", lock.Release)
	}

	// Consumer builds: the four pinned Desktop profiles must exist.
	byProfile := make(map[string]artifact, len(artifacts))
	for _, artifact := range artifacts {
		byProfile[artifact.profile] = artifact
	}
	for _, profile := range desktopProfiles {
		if _, ok := byProfile[profile]; !ok {
			return fmt.Errorf("release %s lacks Desktop profile %s", lock.Release, profile)
		}
	}

	// Consumer builds: Compose files may only reference manifest presets.
	composePresets, err := composePresetNames(root)
	if err != nil {
		return err
	}
	if len(manifest.Presets) > 0 {
		known := make(map[string]bool, len(manifest.Presets))
		for _, preset := range manifest.Presets {
			known[preset.ID] = true
		}
		for _, preset := range composePresets {
			if !known[preset] {
				return fmt.Errorf("deploy/compose references LUMEN_PRESET=%s which release %s does not define", preset, lock.Release)
			}
		}
	}

	// Tag provenance: resolve the release tag to its commit.
	revision, err := resolveTagCommit(lock.Repository, lock.Release)
	if err != nil {
		return err
	}

	catalog, err := generateCatalog(lock.Release, desktopProfiles, byProfile)
	if err != nil {
		return err
	}
	catalogPathAbs := filepath.Join(root, filepath.FromSlash(catalogPath))
	existingCatalog, _ := os.ReadFile(catalogPathAbs)

	staleLock := lock.ManifestSHA256 != manifestHash || lock.Revision != revision
	staleCatalog := !bytesEqual(existingCatalog, []byte(catalog))

	if mode == "check" {
		if staleLock {
			return fmt.Errorf("%s is stale for release %s (manifestSha256 or revision changed); run `task lumen:sync` and commit the result", lockFileName, lock.Release)
		}
		if staleCatalog {
			return fmt.Errorf("%s does not match release %s; run `task lumen:sync` and commit the result", catalogPath, lock.Release)
		}
		fmt.Printf("lumen:check ok — %s catalog, SHA256SUMS, and consumer builds verified for %s\n", lock.Release, catalogPath)
		return nil
	}

	changed := false
	if staleLock {
		lock.ManifestSHA256 = manifestHash
		lock.Revision = revision
		if err := writeLock(lockPath, lock); err != nil {
			return err
		}
		changed = true
		fmt.Printf("lumen:sync updated %s (revision %s, manifestSha256 %s)\n", lockFileName, revision[:12], manifestHash[:12])
	}
	if staleCatalog {
		if err := os.WriteFile(catalogPathAbs, []byte(catalog), 0o644); err != nil {
			return err
		}
		changed = true
		fmt.Printf("lumen:sync regenerated %s from %s\n", catalogPath, lock.Release)
	}
	if !changed {
		fmt.Printf("lumen:sync ok — %s and %s are already in sync\n", lockFileName, catalogPath)
	}
	return nil
}

func parseLock(raw []byte) (lock, error) {
	var parsed lock
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return parsed, fmt.Errorf("decode %s: %w", lockFileName, err)
	}
	if parsed.SchemaVersion != 1 {
		return parsed, fmt.Errorf("%s must use schemaVersion 1", lockFileName)
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
	if !sha256Re.MatchString(parsed.ManifestSHA256) {
		return parsed, fmt.Errorf("%s manifestSha256 must be a lowercase SHA-256", lockFileName)
	}
	return parsed, nil
}

func parseManifest(raw []byte) (manifest, error) {
	var parsed manifest
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return parsed, fmt.Errorf("decode release manifest: %w", err)
	}
	if parsed.SchemaVersion != nil && *parsed.SchemaVersion > 2 {
		return parsed, fmt.Errorf("release manifest schemaVersion %d is newer than supported", *parsed.SchemaVersion)
	}
	if !releaseRe.MatchString(parsed.Version) {
		return parsed, fmt.Errorf("release manifest has an invalid version %q", parsed.Version)
	}
	if len(parsed.Hub) == 0 {
		return parsed, fmt.Errorf("release manifest has no hub artifacts")
	}
	for _, entry := range parsed.Hub {
		if entry.Profile == "" || entry.FileName == "" || entry.URL == "" || !sha256Re.MatchString(entry.Sha256) {
			return parsed, fmt.Errorf("release manifest has an invalid artifact entry")
		}
	}
	return parsed, nil
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
			checks[fields[1]] = fields[0]
		}
		if name == legacyChecksName {
			fmt.Printf("note: release %s published %s (pre-SHA256SUMS release)\n", release, legacyChecksName)
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
	// Prefer the peeled ref (annotated tag -> commit).
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.HasSuffix(fields[1], "^{}") {
			return fields[0], nil
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			return fields[0], nil
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

func generateCatalog(release string, profiles []string, byProfile map[string]artifact) (string, error) {
	sorted := append([]string(nil), profiles...)
	sort.Strings(sorted)

	var builder strings.Builder
	fmt.Fprintf(&builder, "// Code generated by `task lumen:sync` from the Lumen-Hub release catalog.\n")
	fmt.Fprintf(&builder, "// Source: release %s. DO NOT EDIT.\n\n", release)
	builder.WriteString("package lumen\n\n")
	fmt.Fprintf(&builder, "// OfficialReleaseVersion is deliberately pinned to the Lumen Hub release\n// catalog embedded in the Desktop binary. Updating Lumen is an explicit\n// Desktop release change, not a mutable \"latest\" download at runtime.\nconst OfficialReleaseVersion = %q\n\n", release)
	builder.WriteString("var officialReleaseArtifacts = map[string]ReleaseArtifact{\n")
	for _, profile := range sorted {
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

func writeLock(path string, value lock) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".lumen.lock.json-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func fetch(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     true,
			ResponseHeaderTimeout: 30 * time.Second,
		},
	}
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

func sha256Hex(bytes []byte) string {
	sum := sha256.Sum256(bytes)
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
