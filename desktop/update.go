package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/mod/semver"
)

// updateReleasesURL lists releases (newest first), including prereleases. The
// /releases/latest endpoint is deliberately not used because it excludes
// prereleases, and the app currently ships beta (prerelease) tags.
const updateReleasesURL = "https://api.github.com/repos/EdwinZhanCN/Lumilio-Photos/releases?per_page=30"

// updateInfo is a newer release the user can install.
type updateInfo struct {
	Version string // release tag, e.g. "v1.0.0-beta.4"
	URL     string // release page to open in the browser
}

// releaseItem is the subset of the GitHub release JSON the updater reads.
type releaseItem struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Draft   bool   `json:"draft"`
}

// checkForUpdate asks GitHub for the newest release and returns it when it is
// semver-greater than current. current is the build version (may be "dev" or
// lack a leading "v"). Any failure — offline, rate-limited, unparseable — yields
// ok=false: update checks are best-effort, never block, and never surface errors
// (local-first, offline-friendly).
func checkForUpdate(ctx context.Context, current string) (updateInfo, bool) {
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

	return newestUpdate(cur, releases)
}

// newestUpdate picks the highest-semver non-draft release strictly newer than
// cur (canonical "vX.Y.Z[-pre]"). Split out from the HTTP call so it is unit
// testable without a network.
func newestUpdate(cur string, releases []releaseItem) (updateInfo, bool) {
	best := cur
	var found updateInfo
	for _, r := range releases {
		if r.Draft {
			continue
		}
		v := canonicalSemver(r.TagName)
		if v == "" {
			continue
		}
		if semver.Compare(v, best) > 0 {
			best = v
			found = updateInfo{Version: r.TagName, URL: r.HTMLURL}
		}
	}
	return found, found.URL != ""
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
