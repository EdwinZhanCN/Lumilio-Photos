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
~/Library/Application Support/Lumilio Photos/
├── lumilio.lock                      # flock file
├── postgres/
│   └── 16/
│       ├── data/                     # PG data directory
│       ├── run/                      # Unix socket (.s.PGSQL.5487)
│       └── logs/                     # PG logs
├── storage/                          # Photo storage (= STORAGE_PATH)
│   └── primary/
├── secrets/
│   ├── db_password
│   └── lumilio_secret_key
├── config/
│   └── server.local.toml            # Generated runtime config
└── backups/                          # pg_dump auto-backups
```

## Startup Sequence

```
 1. Resolve app data path
    macOS: ~/Library/Application Support/Lumilio Photos/
    Ensure directory tree exists (MkdirAll)

 2. Acquire flock(lumilio.lock, LOCK_EX | LOCK_NB)
    Fail → dialog "Lumilio Photos is already running" → exit

 3. Detect first run (data/ does not exist)

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
    path = "<appdata>/storage"

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
4. **`Makefile`**: Add `desktop-dev` and `desktop-build` targets.

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

## Risk Notes

- **Wails v3 maturity**: Still in alpha. Architecture intentionally avoids deep Wails binding dependency (UI stays on HTTP), so API changes have limited blast radius.
- **App bundle size**: PG 16 + pgvector ≈ 40-60MB per arch. Total app ≈ 100-150MB. Acceptable for a photo management app.
- **First launch time**: initdb + migrations ≈ 5-10s. Needs a splash/progress screen, not a blank window.
