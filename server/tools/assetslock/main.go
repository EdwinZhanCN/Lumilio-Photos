// Command assetslock owns the Lumilio-Assets pin (assets.lock.json).
//
//   - `reconcile` selects the newest stable `assets-vX.Y.Z` release (or an
//     explicit `--release`), resolves the tag's commit SHA, hashes the raw
//     `assets.json` catalog served at that tag, and updates the lock's three
//     derived fields (`release`, `revision`, `manifestSha256`) in one write.
//     Downgrades are rejected by default. Running again on the same release is
//     a no-op.
//   - `check` verifies the committed lock against the pinned revision and
//     release without writing anything; it is the CI gate that validates
//     reconcile PRs.
//
// Assets ships no per-release metadata file; every release is verified by
// `node scripts/verify.mjs` before the tag is pushed, and consumers
// reconstruct the three derived fields from the tag itself.

package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	lockFileName = "assets.lock.json"
	userAgent    = "Lumilio-Photos-assetslock"
)

var (
	repoRe      = regexp.MustCompile(`^https://github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(\.git)?$`)
	tagRe       = regexp.MustCompile(`^assets-v([0-9]+)\.([0-9]+)\.([0-9]+)$`)
	sha256Re    = regexp.MustCompile(`^[a-f0-9]{64}$`)
	sha40Re     = regexp.MustCompile(`^[a-f0-9]{40}$`)
	profileRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	lsRemoteTag = regexp.MustCompile(`refs/tags/assets-v[0-9]+\.[0-9]+\.[0-9]+$`)
)

type lock struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Repository     string `json:"repository"`
	Revision       string `json:"revision"`
	Release        string `json:"release"`
	Profile        string `json:"profile"`
	ManifestSHA256 string `json:"manifestSha256"`
}

type releaseMetadata struct {
	Release        string
	Revision       string
	ManifestSHA256 string
}

type version struct {
	major, minor, patch int
}

func main() {
	if len(os.Args) < 3 || (os.Args[1] != "reconcile" && os.Args[1] != "check") {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/assetslock <reconcile|check> <repository-root> [--release assets-vX.Y.Z]")
		os.Exit(2)
	}
	mode := os.Args[1]
	root, err := filepath.Abs(os.Args[2])
	if err != nil {
		fail(err)
	}
	target, err := parseReleaseArg(os.Args[3:])
	if err != nil {
		fail(err)
	}
	if err := run(mode, root, target); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "assetslock:", err)
	os.Exit(1)
}

func run(mode, root, target string) error {
	lockPath := filepath.Join(root, lockFileName)
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", lockPath, err)
	}
	current, err := parseLock(raw)
	if err != nil {
		return err
	}
	owner, repo, err := repoParts(current.Repository)
	if err != nil {
		return err
	}

	switch mode {
	case "reconcile":
		release := target
		if release == "" {
			release, err = newestStableTag(current.Repository)
			if err != nil {
				return err
			}
		}
		metadata, err := resolveReleaseMetadata(owner, repo, release)
		if err != nil {
			return err
		}

		comparison, err := compareVersions(current.Release, release)
		if err != nil {
			return err
		}
		if comparison > 0 {
			return fmt.Errorf("downgrade rejected: lock is at %s but target is %s", current.Release, release)
		}
		if comparison == 0 &&
			current.Revision == metadata.Revision &&
			current.ManifestSHA256 == metadata.ManifestSHA256 {
			fmt.Printf("assets:reconcile ok — lock already at %s\n", current.Release)
			return nil
		}

		updated := current
		updated.Release = metadata.Release
		updated.Revision = metadata.Revision
		updated.ManifestSHA256 = metadata.ManifestSHA256
		if err := writeLock(lockPath, updated); err != nil {
			return err
		}
		fmt.Printf("assets:reconcile updated %s to %s (revision %s, manifestSha256 %s)\n",
			lockFileName, updated.Release, updated.Revision[:12], updated.ManifestSHA256[:12])
		return nil

	case "check":
		// The pinned revision must serve the exact catalog hash we locked.
		if err := verifyCatalogAt(owner, repo, current.Revision, current.ManifestSHA256); err != nil {
			return err
		}
		// The tag must point at the locked revision.
		tagCommit, err := resolveTagCommit(current.Repository, current.Release)
		if err != nil {
			return err
		}
		if tagCommit != current.Revision {
			return fmt.Errorf("tag %s points at %s but lock revision is %s; run `task assets:reconcile`",
				current.Release, tagCommit, current.Revision)
		}
		fmt.Printf("assets:check ok — %s verified against %s\n", lockFileName, current.Release)
		return nil
	}
	return errors.New("unreachable")
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
	if !tagRe.MatchString(parsed.Release) {
		return parsed, fmt.Errorf("%s release must be a stable tag like assets-v1.2.3", lockFileName)
	}
	if !sha40Re.MatchString(parsed.Revision) {
		return parsed, fmt.Errorf("%s revision must be a full 40-character commit SHA", lockFileName)
	}
	if !sha256Re.MatchString(parsed.ManifestSHA256) {
		return parsed, fmt.Errorf("%s manifestSha256 must be a lowercase SHA-256", lockFileName)
	}
	if !profileRe.MatchString(parsed.Profile) {
		return parsed, fmt.Errorf("%s profile is invalid", lockFileName)
	}
	return parsed, nil
}

func parseReleaseArg(args []string) (string, error) {
	var target string
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "--release":
			if index+1 >= len(args) {
				return "", errors.New("--release requires a value")
			}
			index++
			target = args[index]
		default:
			return "", fmt.Errorf("unknown argument: %s", args[index])
		}
	}
	if target != "" && !tagRe.MatchString(target) {
		return "", fmt.Errorf("--release must be a stable tag like assets-v1.2.3")
	}
	return target, nil
}

func repoParts(repository string) (owner, repo string, err error) {
	match := repoRe.FindStringSubmatch(repository)
	if match == nil {
		return "", "", fmt.Errorf("invalid repository URL %q", repository)
	}
	return match[1], match[2], nil
}

// newestStableTag lists remote tags and picks the highest stable assets-v tag.
func newestStableTag(repository string) (string, error) {
	output, err := exec.Command("git", "ls-remote", "--tags", repository).Output()
	if err != nil {
		return "", fmt.Errorf("list tags of %s: %w", repository, err)
	}
	best := ""
	var bestVersion version
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !lsRemoteTag.MatchString(fields[1]) {
			continue
		}
		tag := strings.TrimPrefix(fields[1], "refs/tags/")
		parsed, err := parseVersion(tag)
		if err != nil {
			continue
		}
		if best == "" || parsed.greaterThan(bestVersion) {
			best = tag
			bestVersion = parsed
		}
	}
	if best == "" {
		return "", fmt.Errorf("no stable assets-v release tags found for %s", repository)
	}
	return best, nil
}

func parseVersion(tag string) (version, error) {
	match := tagRe.FindStringSubmatch(tag)
	if match == nil {
		return version{}, fmt.Errorf("not a stable assets tag: %s", tag)
	}
	var parts [3]int
	for index := 0; index < 3; index++ {
		value, err := strconv.Atoi(match[index+1])
		if err != nil {
			return version{}, err
		}
		parts[index] = value
	}
	return version{major: parts[0], minor: parts[1], patch: parts[2]}, nil
}

func (left version) greaterThan(right version) bool {
	if left.major != right.major {
		return left.major > right.major
	}
	if left.minor != right.minor {
		return left.minor > right.minor
	}
	return left.patch > right.patch
}

// compareVersions returns -1, 0, or 1 comparing two assets-v tags.
func compareVersions(leftTag, rightTag string) (int, error) {
	left, err := parseVersion(leftTag)
	if err != nil {
		return 0, err
	}
	right, err := parseVersion(rightTag)
	if err != nil {
		return 0, err
	}
	switch {
	case left.greaterThan(right):
		return 1, nil
	case right.greaterThan(left):
		return -1, nil
	default:
		return 0, nil
	}
}

// resolveReleaseMetadata reconstructs the release metadata for a tag: the
// commit SHA the tag points at and the SHA-256 of assets.json served at that
// tag. There is no per-release metadata file to read.
func resolveReleaseMetadata(owner, repo, release string) (releaseMetadata, error) {
	catalogRaw, catalogErr := fetch(fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/assets.json", owner, repo, release))
	if catalogErr != nil {
		return releaseMetadata{}, fmt.Errorf("release %s is not reachable as a tag (%v)", release, catalogErr)
	}
	revision, tagErr := resolveTagCommit(fmt.Sprintf("https://github.com/%s/%s.git", owner, repo), release)
	if tagErr != nil {
		return releaseMetadata{}, fmt.Errorf("release %s: %w", release, tagErr)
	}
	return releaseMetadata{
		Release:        release,
		Revision:       revision,
		ManifestSHA256: sha256Hex(catalogRaw),
	}, nil
}

// verifyCatalogAt checks that the assets.json served at ref hashes to want.
func verifyCatalogAt(owner, repo, ref, want string) error {
	raw, err := fetch(fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/assets.json", owner, repo, ref))
	if err != nil {
		return fmt.Errorf("fetch assets.json at %s: %w", ref, err)
	}
	hash := sha256Hex(raw)
	if hash != want {
		return fmt.Errorf("assets.json at %s hashes to %s but the lock expects %s", ref, hash, want)
	}
	return nil
}

func resolveTagCommit(repository, release string) (string, error) {
	output, err := exec.Command("git", "ls-remote", "--tags", repository, "refs/tags/"+release, "refs/tags/"+release+"^{}").Output()
	if err != nil {
		return "", fmt.Errorf("resolve tag %s: %w", release, err)
	}
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

func writeLock(path string, value lock) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".assets.lock.json-*")
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
	return fmt.Sprintf("%x", sum)
}
