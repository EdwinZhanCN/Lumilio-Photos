# Desktop Distribution — Wails v3 + Bundled PostgreSQL

**Goal**: Ship a macOS desktop app that bundles a private PostgreSQL 16 runtime, manages its lifecycle, and reuses the existing Go API server and React UI over HTTP/OpenAPI.

**Status**: Planning

---

## Architecture

```
Wails v3 app
  → Go desktop supervisor
      → 管理私有 PostgreSQL 16 runtime
      → initdb / start / stop / health check
      → 生成 server.local.toml
      → 跑 Lumilio migrations + River migrations (Go API)
      → 启动现有 Go API server (in-process)
      → 管理 mDNS ML discovery
  → React UI
      → 继续走 HTTP/OpenAPI（连接 127.0.0.1:6680）
```

Single binary. Desktop supervisor imports server packages directly via Go workspace, no subprocess.

## macOS Signing (No Apple Developer Account)

| Channel | Signing | User Friction |
|---|---|---|
| **Homebrew Cask** (primary) | None needed | `brew install --cask lumilio-photos` |
| **GitHub Releases DMG** (secondary) | Ad-hoc (`codesign -s -`, free) | Right-click → Open once |

First-launch quarantine cleanup for bundled PG binaries:

```go
// quarantine_darwin.go (build tag: darwin)
exec.Command("xattr", "-dr", "com.apple.quarantine", resourcesDir).Run()
```

Main process (already user-trusted) strips quarantine from Resources directory on first run, so subsequent `exec` calls to pg_ctl/initdb are not blocked by Gatekeeper.

## Project Structure

```
desktop/                              # New top-level directory
├── main.go                           # Wails v3 entry
├── app.go                            # Wails lifecycle hooks
├── supervisor/
│   ├── supervisor.go                 # Orchestrator: startup/shutdown sequence
│   ├── postgres.go                   # initdb / pg_ctl / pg_isready / createdb
│   ├── config.go                     # Generates server.local.toml for desktop
│   ├── lock.go                       # flock single-instance guard
│   ├── paths.go                      # OS-specific app data paths
│   └── quarantine_darwin.go          # xattr cleanup (build-tagged)
├── resources/
│   └── postgres/16/
│       ├── darwin-arm64/             # Apple Silicon
│       │   ├── bin/                  # postgres, initdb, pg_ctl, pg_isready,
│       │   │                         # pg_dump, pg_restore, createdb
│       │   ├── lib/
│       │   └── share/postgresql/
│       └── darwin-amd64/             # Intel (if supported)
└── wails.json
```

**No `bin/river`** in resources — River migrations already use Go API (`rivermigrate`).

### Go Module Structure

```
# Root go.work
go 1.24

use (
    ./server
    ./desktop
)
```

## Runtime Data Directory

```
~/Library/Application Support/Lumilio Photos/   # APP DATA — always local, never user-relocatable
├── lumilio.lock                      # flock file
├── postgres/
│   └── 16/
│       ├── data/                     # PG data directory (MUST stay on local disk)
│       ├── run/                      # Unix socket (.s.PGSQL.5487)
│       └── logs/                     # PG logs
├── secrets/                          # MUST stay local (decouple from storage — see below)
│   ├── db_password
│   └── lumilio_secret_key
├── config/
│   ├── server.local.toml            # Generated runtime config (rewritten every launch)
│   └── desktop-settings.json        # Persisted user choices (NOT rewritten) — e.g. storage_path
└── backups/                          # pg_dump auto-backups

<storage_path>/                       # USER-CHOSEN library location (default: <appdata>/storage)
└── primary/                          # repo structure created by the server on startup
```

Default `<storage_path>` is `<appdata>/storage`, but the user may relocate it to
any path (external drive, custom folder) — like a docker volume mount. PG data and
secrets are deliberately NOT under `<storage_path>` so the library can move freely.
See "User-Selectable Storage Path" below.

```
```

## Startup Sequence

```
 1. Resolve app data path
    macOS: ~/Library/Application Support/Lumilio Photos/
    Ensure directory tree exists (MkdirAll)

 2. Acquire flock(lumilio.lock, LOCK_EX | LOCK_NB)
    Fail → dialog "Lumilio Photos is already running" → exit

 3. Detect first run (data/ does not exist)

 3a. Resolve storage path (see "User-Selectable Storage Path")
    Read <appdata>/config/desktop-settings.json → storage_path
    [First run · no setting yet] Onboarding: default <appdata>/storage,
      offer native directory picker to relocate; persist choice to
      desktop-settings.json (NOT to server.local.toml — that gets rewritten)
    [Subsequent] Use persisted storage_path
    Verify reachable (external drive may be unmounted) → friendly dialog +
      "use default location" escape if not; never crash
    MkdirAll <storage_path>

 4. [First run · macOS] Strip quarantine
    xattr -dr com.apple.quarantine <bundle>/Contents/Resources/postgres/

 5. [First run] initdb
    <resources>/bin/initdb \
      -D <appdata>/postgres/16/data \
      -U lumilio \
      --auth=trust \
      --encoding=UTF8 \
      --locale=C

 6. [First run] Generate secrets
    db_password:        32-byte crypto/rand → hex → write file
    lumilio_secret_key: same

 7. Write postgresql.conf (overwrite every launch to stay in sync)
    listen_addresses = ''              # Unix socket only, no TCP port
    unix_socket_directories = '<appdata>/postgres/16/run'
    port = 5487                        # Non-standard; only affects socket filename
    shared_buffers = 32MB
    work_mem = 4MB
    maintenance_work_mem = 16MB
    max_connections = 10
    wal_level = minimal
    max_wal_senders = 0
    logging_collector = on
    log_directory = '<appdata>/postgres/16/logs'
    log_filename = 'postgresql-%Y-%m-%d.log'
    log_rotation_age = 1d
    log_rotation_size = 10MB

 8. Write pg_hba.conf
    local  all  lumilio  trust         # Unix socket, no password needed

 9. Handle stale state
    Check postmaster.pid → PID alive?
    Alive → pg_ctl stop -m fast (previous crash leftover)
    Dead  → remove stale pid file

10. pg_ctl start
    <resources>/bin/pg_ctl start \
      -D <appdata>/postgres/16/data \
      -l <appdata>/postgres/16/logs/postgres.log \
      -w

11. pg_isready health check (exponential backoff, 30s timeout)
    <resources>/bin/pg_isready \
      -h <appdata>/postgres/16/run \
      -p 5487

12. createdb (idempotent)
    Connect to postgres db, CREATE DATABASE IF NOT EXISTS lumiliophotos

13. Run migrations
    Reuse server/internal/db.AutoMigrate()
    → golang-migrate (app migrations)
    → rivermigrate Go API (River schema)

14. Generate server.local.toml
    [server]
    port = "6680"
    log_level = "info"
    cors_allowed_origins = ["http://localhost:6657"]

    [database]
    host = "<appdata>/postgres/16/run"   # Unix socket directory
    port = "5487"
    user = "lumilio"
    password_file = "<appdata>/secrets/db_password"
    name = "lumiliophotos"
    ssl = "disable"

    [storage]
    path = "<storage_path>"              # from step 3a (default <appdata>/storage)

    [ml]
    clip_enabled = true
    bioclip_enabled = true
    ocr_enabled = true
    face_enabled = true

    [auth]
    secret_key_path = "<appdata>/secrets/lumilio_secret_key"
    # WebAuthn: pin RP ID/origin to localhost. See "WebAuthn / Passkey
    # Constraints" below — these are hard requirements, not defaults.
    webauthn_rp_id = "localhost"
    webauthn_rp_origins = ["http://localhost:6680"]

    [lumen]
    discovery_enabled = true
    discovery_mdns_enabled = true        # Desktop default: mDNS on
    discovery_hub_url = ""

15. Start API server (in-process)
    Import server bootstrap, pass config path

16. Wails ready → open UI window
    Webview MUST navigate to http://localhost:6680
    (NOT 127.0.0.1, NOT a Wails custom scheme — see WebAuthn constraints)
```

## Shutdown Sequence

```
1. Wails OnShutdown / SIGTERM / SIGINT
2. context.Cancel → API server graceful shutdown (drain connections, 10s max)
3. pg_ctl stop -D <data> -m fast (30s timeout)
4. Release flock
5. Exit
```

Force-quit protection: next startup handles stale postmaster.pid at step 9.

## PostgreSQL Configuration Rationale

| Parameter | Value | Reason |
|---|---|---|
| `listen_addresses` | `''` (empty) | Unix socket only; zero TCP port conflicts |
| `port` | `5487` | Only affects socket filename, not a real port |
| `shared_buffers` | `32MB` | Desktop, not a server |
| `max_connections` | `10` | API server + migrations is enough |
| `wal_level` | `minimal` | No replication needed |
| `max_wal_senders` | `0` | Same |

## Unix Socket Path Length

macOS limits socket paths to ~104 bytes. Worst case:

```
/Users/<long-username>/Library/Application Support/Lumilio Photos/postgres/16/run/.s.PGSQL.5487
```

~95 bytes with a typical username. Safety: if path > 90 bytes, fall back to `/tmp/lumilio-<uid>/pg.sock` and point toml at it.

## PG Major Version Upgrade Path

Directory structure encodes the version (`postgres/16/`). Upgrading to PG 17:

```
1. App startup detects: bundled version (17) ≠ data version (16)
2. Dialog: "Upgrading database, do not close the app"
3. Auto pg_dump → backups/pre-upgrade-<timestamp>.sql
4. initdb new data dir → postgres/17/data/
5. pg_restore from backup
6. Keep postgres/16/ until next startup, then clean up
```

## User-Selectable Storage Path

The photo library location (`[storage].path`) is user-selectable on desktop, like
a docker volume mount. Everything else (PG data, secrets) stays pinned to local app
data. This gives users the flexibility to keep their (potentially huge) media on an
external drive or a custom folder, without putting the database at risk.

### What is and isn't relocatable

| Path | Location | Why |
|---|---|---|
| `[storage].path` (media library) | **User-chosen**, default `<appdata>/storage` | Large, user may want it on an external drive |
| PG data dir | **Always** `<appdata>/postgres/16/data` | PostgreSQL on network/external volumes has fsync + file-lock risk; drive unmount = DB crash |
| secrets (`db_password`, `secret_key`) | **Always** `<appdata>/secrets/` | If secrets followed storage to an unmounted external drive, PG/auth couldn't start |

### Deliberate divergence from docker convention

In docker, `db_password` defaults to **under** the storage volume
(`config.go` `DBPasswordFilePath()` → `data/storage/.secrets/db_password`,
`docker-compose.yml` `LUMILIO_DB_PASSWORD_FILE: /data/storage/.secrets/db_password`,
intentionally "persists alongside the user's media").

**Desktop deliberately does NOT follow this.** Secrets move to `<appdata>/secrets/`,
decoupled from storage. Reason: storage is relocatable to an external drive; if the
secret lived under storage and the drive were unmounted, PG couldn't read its
password and would fail to start. Do not "fix" desktop back to the docker
convention — the split is intentional and load-bearing.

### Persistence

The user's storage choice must survive across launches, but `server.local.toml` is
regenerated every launch (startup step 14), so it cannot be the source of truth.

```
<appdata>/config/desktop-settings.json   # { "storage_path": "/Volumes/Photos/Lumilio" }
```

- **First run**: onboarding defaults to `<appdata>/storage`; offer a native
  directory picker (`application.OpenFileDialog().CanChooseDirectories(true)`,
  native — not webui) to relocate. Persist the result to `desktop-settings.json`.
- **Subsequent runs**: read `desktop-settings.json`, inject into the generated toml.

Don't force the picker — defaulting reduces first-launch friction; most users keep
the default. Surface "choose another location" as an option, not a gate.

### No backend change needed

`[storage].path` / `STORAGE_PATH` already exist. The supervisor just injects the
resolved path when generating the toml.

### Edge cases

| Case | Handling |
|---|---|
| External drive unmounted at startup | Check reachability (step 3a); friendly "please connect the storage drive" dialog + "use default location" escape; never crash |
| macOS TCC permission | Native picker selection grants access (powerbox); selecting `~/Documents`/external volumes on macOS 15 may still trigger a system prompt. Low risk for non-sandboxed ad-hoc app |
| User wants to move library later | v1: first-run choice only. Relocation (move files + update settings + restart) is a settings-page feature for phase 2 |

## Native Dependencies Bundling

The server has four native runtime dependencies (see `server/Dockerfile`). On
desktop there is no apt/PATH to rely on — everything must ship inside the app
bundle. They split into two integration models with very different bundling work.

| Dependency | Integration | Code location | Bundle as |
|---|---|---|---|
| **libvips** | cgo compile-time link (`govips/v2`) | `internal/utils/imaging/process.go` | dylib tree in `Contents/Frameworks/` |
| **libraw** | NOT called directly — libvips' RAW load delegate | (Dockerfile note) | comes free with libvips tree |
| **exiftool** | subprocess, hardcoded to PATH | `internal/utils/exif/extract.go:222` | standalone dist in `Resources/` |
| **ffmpeg** | subprocess (transcode) | — | static binary in `Resources/` |

### Track A — libvips + libraw + dependency tree (linked libs)

govips dynamically links `libvips.42.dylib`, which drags a large transitive tree:
glib, gobject, jpeg, png, webp, tiff, libheif, lcms2, libexif, orc, fftw, etc.
**libraw needs no special handling** — it's a libvips delegate, so it rides along
with the tree as long as the bundled libvips was built with libraw support
(Homebrew's `vips` includes it by default).

macOS bundling flow:

```
1. brew install vips            # pulls libraw/libheif/etc. as deps
2. brew install dylibbundler
3. dylibbundler -od -b \
     -x "build/bin/Lumilio Photos.app/Contents/MacOS/server-binary" \
     -d "build/bin/Lumilio Photos.app/Contents/Frameworks/" \
     -p "@executable_path/../Frameworks/"
   # recursively collects every dylib, rewrites install names to @rpath
4. Go build: -ldflags "-r @executable_path/../Frameworks"  (set rpath)
5. codesign -s - each dylib in Frameworks/  (install_name rewrite invalidates
   the original signature — dozens of dylibs, all need ad-hoc re-signing)
```

This is the second-largest engineering chunk after PG lifecycle. The ad-hoc
re-signing stacks with the signing strategy above.

### Track B — exiftool + ffmpeg (subprocess binaries)

Simpler, but **requires code changes**. Currently `internal/utils/exif/extract.go`
and `internal/utils/exif/utils.go` hardcode `exec.Command(ctx, "exiftool", ...)`
and `LookPath("exiftool")` against system PATH. Desktop has no system exiftool, so
the path must become configurable.

- **exiftool**: ship the official macOS standalone build (bundles its own Perl, no
  system Perl dependency) under `Resources/exiftool/`. ~6MB.
- **ffmpeg**: ship a static build (BtbN / evermeet.cx macOS build) under
  `Resources/ffmpeg/`. ~70-80MB with full codecs. VideoToolbox HW transcode works
  in static builds.

**Required code change** (must not break web/docker):
- Add optional config/env overrides `EXIFTOOL_PATH` and `FFMPEG_PATH`.
- Empty default = current behavior (resolve via PATH) → web/docker unchanged.
- Desktop supervisor injects bundle-internal absolute paths at startup.
- This introduces an "external tool path" abstraction layer in server config that
  only desktop uses; design it as an optional override, not a new requirement.

### Bundle Size Impact

| Component | Size (arm64) |
|---|---|
| PG 16 + pgvector | 40-60MB |
| libvips dylib tree | 30-50MB |
| ffmpeg static | 70-80MB |
| exiftool | ~6MB |
| Go binary + Wails webview | ~30MB |
| **Total** | **~180-230MB** |

Acceptable for a photo app, but **ship separate arm64 + amd64 packages, not a
universal binary** (universal nearly doubles the size).

## WebAuthn / Passkey Constraints (HARD REQUIREMENTS)

Passkeys still work on desktop, but two constraints are non-negotiable. Violating
either silently breaks passkey registration/login.

The backend derives RP ID dynamically: `resolveWebAuthnRPID` in
`server/internal/service/auth_passkeys.go` falls back to the request origin's
host when `webauthn_rp_id` is unset. So the backend is not the bottleneck — the
webview origin and the protocol's domain-binding rules are.

**Constraint 1 — Webview navigation target.**
The desktop webview MUST navigate to `http://localhost:6680`.
- NOT `127.0.0.1` — WebAuthn rejects IP addresses as RP IDs; `localhost` is the
  one allowed special case (also treated as a secure context over plain HTTP).
- NOT a Wails custom scheme (`wails://`, `http://wails.localhost`, etc.) — that
  produces a non-standard origin and WebAuthn fails or derives the wrong RP ID.
- Do NOT serve the React bundle via Wails' asset server. Let the Go API server
  serve the SPA and point the webview at it over HTTP.

**Constraint 2 — Pin RP ID/origin in `server.local.toml`.**
```
[auth]
webauthn_rp_id = "localhost"
webauthn_rp_origins = ["http://localhost:6680"]
```
Functionally equivalent to leaving `webauthn_rp_id` empty (host derivation yields
`localhost` anyway), but pinning it acts as an assertion: if the webview origin is
ever misconfigured, registration fails loudly with
`origin host X does not match configured rp id localhost` instead of silently
registering a passkey under the wrong RP that can't be found at next login.

**Accepted fact — passkeys are NOT portable across deployments.**
A passkey is bound to its RP ID. Web deployment uses RP ID = the user's domain
(e.g. `photos.example.com`); desktop uses RP ID = `localhost`. These are distinct
Relying Parties, so a passkey registered on web cannot be used on desktop and vice
versa. Users must register a passkey separately on each. This is acceptable because
passkey is an optional enhancement (TOTP is the MFA baseline, see
mfa-hardening-plan) — desktop users can always log in with password + TOTP, then
register a desktop-local passkey.

**Open risk — WKWebView platform authenticator.**
Whether embedded WKWebView (macOS) will surface the platform authenticator
(Touch ID / iCloud Keychain passkey) under a `localhost` RP ID with ad-hoc signing
is unverified. Embedded WKWebView historically required an Associated Domains
entitlement + hosted AASA for platform passkeys, which needs an Apple Developer
account. Spike this before committing: a ~20-line test calling
`navigator.credentials.create()` in the Wails webview against the local server.
Fallbacks if it doesn't surface: cross-device/hybrid passkeys (phone QR) or
security keys may still work; worst case is password + TOTP, which is sufficient.

## Existing Code Changes Needed

1. **`server/cmd/main.go`**: Extract bootstrap logic into a callable function so desktop supervisor can import and invoke it (currently only `func main()`).
2. **`server/config/config.go`**: Already supports absolute paths via `SERVER_CONFIG_FILE` env var — sufficient for desktop mode.
3. **`server/internal/db/migration.go`**: `AutoMigrate()` already standalone — reuse directly.
4. **`server/internal/utils/exif/*` + transcode**: Add optional `EXIFTOOL_PATH` / `FFMPEG_PATH` config/env overrides (empty = resolve via PATH, preserving web/docker). Replace hardcoded `exec.Command("exiftool", ...)` with the resolved path. See "Native Dependencies Bundling → Track B".
5. **`Makefile`**: Add `desktop-dev` and `desktop-build` targets.

## Build Pipeline

### PG Binaries (CI)

Build from source in GitHub Actions for full control:

```yaml
# .github/workflows/build-postgres.yml
- name: Build PostgreSQL 16 + pgvector
  run: |
    curl -O https://ftp.postgresql.org/pub/source/v16.9/postgresql-16.9.tar.bz2
    tar xf postgresql-16.9.tar.bz2
    cd postgresql-16.9
    ./configure --prefix=$PWD/../pg-dist --without-readline --without-zlib
    make -j$(nproc) && make install
    cd ../pgvector
    make PG_CONFIG=../pg-dist/bin/pg_config install
```

### App Build

```bash
cd desktop
wails3 build -platform darwin/arm64
codesign --force --deep -s - "build/bin/Lumilio Photos.app"
hdiutil create -volname "Lumilio Photos" \
  -srcfolder "build/bin/Lumilio Photos.app" \
  -ov -format UDZO "Lumilio-Photos-arm64.dmg"
```

### Homebrew Cask Formula

```ruby
cask "lumilio-photos" do
  version "1.0.0"
  sha256 "..."
  url "https://github.com/EdwinZhanCN/Lumilio-Photos/releases/download/v#{version}/Lumilio-Photos-#{arch}.dmg"
  name "Lumilio Photos"
  homepage "https://github.com/EdwinZhanCN/Lumilio-Photos"
  app "Lumilio Photos.app"
  zap trash: "~/Library/Application Support/Lumilio Photos"
end
```

## Validation

- [ ] `supervisor/postgres.go`: initdb → start → pg_isready → createdb → stop cycle works
- [ ] Quarantine cleanup: PG binaries execute after first-launch xattr strip
- [ ] Crash recovery: kill -9 postgres, next launch recovers cleanly
- [ ] Socket path: works with long usernames (or falls back to /tmp)
- [ ] Shutdown ordering: API server drains before PG stops
- [ ] Ad-hoc signed DMG: right-click → Open works on clean macOS
- [ ] Homebrew cask: install/uninstall/zap work
- [ ] PG upgrade: 16→17 dump/restore path (manual test when relevant)
- [ ] Webview origin is `http://localhost:6680` (not 127.0.0.1, not custom scheme)
- [ ] Passkey registration succeeds in the desktop webview (or spike documents the WKWebView limitation + chosen fallback)
- [ ] libvips dylib tree bundled via dylibbundler, @rpath resolves, all dylibs ad-hoc signed
- [ ] RAW decode works (confirms libraw delegate rode along with libvips)
- [ ] exiftool + ffmpeg resolve from bundled `Resources/` paths (EXIFTOOL_PATH/FFMPEG_PATH), web/docker still use PATH
- [ ] Storage picker: first-run choice persists to desktop-settings.json, survives relaunch, default path works without picker
- [ ] Storage on external drive: unmounted-at-startup shows friendly dialog (no crash), PG still starts (secrets/data are local)

## Risk Notes

- **Wails v3 maturity**: Still in alpha. Architecture intentionally avoids deep Wails binding dependency (UI stays on HTTP), so API changes have limited blast radius.
- **App bundle size**: PG 16 + pgvector ≈ 40-60MB per arch. Total app ≈ 100-150MB. Acceptable for a photo management app.
- **First launch time**: initdb + migrations ≈ 5-10s. Needs a splash/progress screen, not a blank window.
