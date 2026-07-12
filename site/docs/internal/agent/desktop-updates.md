# Desktop app updates

How the Wails desktop host discovers newer releases and gets the user onto the
right installer. Scope is `desktop/` only — Docker / Compose channels are
separate.

Status: **usable for beta**. Checks are best-effort; install is still manual
(open installer URL). Silent / signed auto-update (Sparkle, WinSparkle, etc.)
is deferred until code-signing is affordable.

## What works today

1. After the runtime is ready, the tray host calls `checkForUpdate` in the
   background (`desktop/app.go`).
2. It lists GitHub Releases (including **prereleases** — `/releases/latest` is
   deliberately unused because betas are prerelease tags).
3. It picks the highest semver strictly newer than `main.buildVersion`.
4. It resolves a **platform installer URL**, not the release HTML page:
   - macOS: matching `*-macos-<arch>.dmg` (arm64 / amd64)
   - Windows: `*-windows-*-setup.exe`, then other `.exe`, then `.zip`
5. If nothing matches, it falls back to the release page URL.
6. The tray shows **Update available: \<tag\>** / **有新版本:\<tag\>**. Click
   opens the URL in the default browser; the user installs over the existing
   app. Library + app-data are untouched.

Failures (offline, rate-limit, unparseable version such as `dev`) are silent —
updates never block boot.

## Download region (desktop-only)

Persisted as `DesktopSettings.Region` in `desktop-settings.json`
(`cn` | `other`). Independent of the in-browser preference `region` (maps /
OpenStreetMap and similar).

| Where | Behavior |
|---|---|
| Onboarding (storage step) | Choose download region; saved on complete |
| Dashboard / control panel | Change anytime via `POST /__onb/region` |
| Default when empty | `zh` UI language → `cn`, else `other` |

When region is `cn`, installer (and release-page fallback) URLs that point at
`github.com` or `objects.githubusercontent.com` are rewritten through
`cnGitHubReleaseMirror` in `desktop/update.go` (currently
`https://gh-proxy.com/`). Empty constant disables rewriting.

The **Releases API** check still hits `api.github.com` directly. Only download
URLs are mirrored. The same desktop region is also passed into Lumen model
download config (`lumen.ConfigSelection.Region`).

## Key files

| Path | Role |
|---|---|
| `desktop/update.go` | GitHub list, semver pick, asset match, CN mirror |
| `desktop/update_test.go` | Unit tests (no network) |
| `desktop/app.go` | Background check, tray menu, `desktopRegion()` |
| `desktop/supervisor/config.go` | `Region` field on settings |
| `desktop/onboarding.go` | State / complete / `POST /__onb/region` |
| `desktop/onboarding/index.html` | Region UI (onboarding + dashboard) |
| `desktop/packaging/windows/lumilio.iss` | Inno setup; in-place upgrade clears `{app}\*` |

Release asset naming must keep matching `pickReleaseAssetURL` (see
`release-cicd` plan / `release-desktop.yml`).

## Not in scope yet

- Background download + silent replace (needs Apple/Windows signing).
- Mirroring `api.github.com` itself for mainland checks.
- First-party CDN (Cloudflare Worker / R2 custom domain) — swap
  `cnGitHubReleaseMirror` when ready.
- PostgreSQL major-version upgrade path — see
  `exec-plans/active/db-backup-upgrade.md` (orthogonal to app replace).

## Related

- User-facing install notes: `site/docs/{en,zh-cn}/user-manual/introduction/installation.md`
- Onboarding UX record: `exec-plans/completed/desktop-onboarding.md`
- Desktop module overview: `desktop/README.md`
