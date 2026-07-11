package main

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// updateReleasesURL lists releases (newest first), including prereleases. The
// /releases/latest endpoint is deliberately not used because it excludes
// prereleases, and the app currently ships beta (prerelease) tags.
const updateReleasesURL = "https://api.github.com/repos/EdwinZhanCN/Lumilio-Photos/releases?per_page=30"

// cnGitHubReleaseMirror prefixes GitHub https URLs when desktop region is "cn"
// so mainland users can fetch release assets without hitting github.com
// directly. Trailing slash required. Point this at a Cloudflare Worker / R2
// custom domain when one is ready (e.g. "https://downloads.lumilio.org/").
// Empty disables rewriting.
const cnGitHubReleaseMirror = "https://gh-proxy.com/"

// updateInfo is a newer release the user can install.
type updateInfo struct {
	Version string // release tag, e.g. "v1.0.0-beta.4"
	URL     string // platform installer/asset URL (or release page fallback)
}

// releaseItem is the subset of the GitHub release JSON the updater reads.
type releaseItem struct {
	TagName string         `json:"tag_name"`
	HTMLURL string         `json:"html_url"`
	Draft   bool           `json:"draft"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// checkForUpdate asks GitHub for the newest release and returns it when it is
// semver-greater than current. current is the build version (may be "dev" or
// lack a leading "v"). region selects whether asset URLs are rewritten through
// the mainland mirror. Any failure — offline, rate-limited, unparseable —
// yields ok=false: update checks are best-effort, never block, and never
// surface errors (local-first, offline-friendly).
func checkForUpdate(ctx context.Context, current, region string) (updateInfo, bool) {
	cur := canonicalSemver(current)
	if cur == "" {
		return updateInfo{}, false // "dev" / unparseable → don't nag
	}

	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, updateReleasesURL, nil)
	if err != nil {
		return updateInfo{}, false
	}
	// GitHub requires a User-Agent; the versioned Accept header pins the API shape.
	req.Header.Set("User-Agent", "Lumilio-Photos-Desktop")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return updateInfo{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return updateInfo{}, false
	}

	var releases []releaseItem
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return updateInfo{}, false
	}

	return newestUpdate(cur, releases, runtime.GOOS, runtime.GOARCH, region)
}

// newestUpdate picks the highest-semver non-draft release strictly newer than
// cur and resolves a platform installer URL (DMG / setup.exe). Split out from
// the HTTP call so it is unit testable without a network.
func newestUpdate(cur string, releases []releaseItem, goos, goarch, region string) (updateInfo, bool) {
	best := cur
	var found *releaseItem
	for i := range releases {
		r := &releases[i]
		if r.Draft {
			continue
		}
		v := canonicalSemver(r.TagName)
		if v == "" {
			continue
		}
		if semver.Compare(v, best) > 0 {
			best = v
			found = r
		}
	}
	if found == nil {
		return updateInfo{}, false
	}
	url := pickReleaseAssetURL(found.Assets, goos, goarch)
	if url == "" {
		url = found.HTMLURL
	}
	url = maybeMirrorGitHubURL(url, region)
	return updateInfo{Version: found.TagName, URL: url}, true
}

// pickReleaseAssetURL chooses the installer the current OS should download:
// macOS → .dmg matching arch; Windows → setup.exe preferred over portable zip.
func pickReleaseAssetURL(assets []releaseAsset, goos, goarch string) string {
	lower := make([]releaseAsset, 0, len(assets))
	for _, a := range assets {
		lower = append(lower, releaseAsset{
			Name:               strings.ToLower(a.Name),
			BrowserDownloadURL: a.BrowserDownloadURL,
		})
	}
	switch goos {
	case "darwin":
		archToken := "arm64"
		if goarch == "amd64" {
			archToken = "amd64"
		}
		for _, a := range lower {
			if strings.Contains(a.Name, "macos") && strings.Contains(a.Name, archToken) && strings.HasSuffix(a.Name, ".dmg") {
				return a.BrowserDownloadURL
			}
		}
		for _, a := range lower {
			if strings.HasSuffix(a.Name, ".dmg") {
				return a.BrowserDownloadURL
			}
		}
	case "windows":
		for _, a := range lower {
			if strings.Contains(a.Name, "windows") && strings.HasSuffix(a.Name, "-setup.exe") {
				return a.BrowserDownloadURL
			}
		}
		for _, a := range lower {
			if strings.Contains(a.Name, "windows") && strings.HasSuffix(a.Name, ".exe") {
				return a.BrowserDownloadURL
			}
		}
		for _, a := range lower {
			if strings.Contains(a.Name, "windows") && strings.HasSuffix(a.Name, ".zip") {
				return a.BrowserDownloadURL
			}
		}
	}
	return ""
}

// maybeMirrorGitHubURL rewrites github.com (and release CDN) URLs through the
// mainland mirror when region is "cn".
func maybeMirrorGitHubURL(rawURL, region string) string {
	if normalizeRegion(region) != "cn" || cnGitHubReleaseMirror == "" || rawURL == "" {
		return rawURL
	}
	if !strings.HasPrefix(rawURL, "https://github.com/") &&
		!strings.HasPrefix(rawURL, "https://objects.githubusercontent.com/") {
		return rawURL
	}
	return strings.TrimRight(cnGitHubReleaseMirror, "/") + "/" + rawURL
}

// canonicalSemver normalizes a version/tag ("1.0.0-beta.3", "v1.0.0") to the
// "vX.Y.Z[-pre]" form golang.org/x/mod/semver needs, or "" if it is not valid
// semver (notably the "dev" default of an unstamped build).
func canonicalSemver(s string) string {
	if s == "" {
		return ""
	}
	if s[0] != 'v' {
		s = "v" + s
	}
	if !semver.IsValid(s) {
		return ""
	}
	return s
}

// normalizeRegion returns "cn" or "other".
func normalizeRegion(r string) string {
	switch strings.ToLower(strings.TrimSpace(r)) {
	case "cn", "china", "zh-cn", "zh_cn":
		return "cn"
	default:
		return "other"
	}
}

// defaultRegion picks a desktop region when none is persisted yet.
func defaultRegion(lang string) string {
	if normalizeLang(lang) == "zh" {
		return "cn"
	}
	return "other"
}

// effectiveRegion returns the persisted region or a language-based default.
func effectiveRegion(persisted, lang string) string {
	if strings.TrimSpace(persisted) != "" {
		return normalizeRegion(persisted)
	}
	return defaultRegion(lang)
}
