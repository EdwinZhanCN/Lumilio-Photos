# Lumilio Photos Desktop

This is the Wails v3 desktop host for Lumilio Photos. Server and SQLite run
in-process; the optional Lumen Hub is a separately supervised child process.
The native tray and Settings window consume the same typed control-plane
snapshot.

The versions in `toolchain.json` are part of the build contract. The Wails CLI
must report `3.0.0-alpha2.119`; generated bindings and resource metadata are
checked in after running the pinned command.

Useful local commands:

```sh
cd desktop
vp install
vp run check
vp run build
GOCACHE=/private/tmp/lumilio-desktop-go-cache go test -race ./...
```

The generated TypeScript bindings live in `frontend/bindings` and are produced
with:

```sh
wails3 generate bindings -f '' -clean=true -ts -i
```

Runtime intent, journals, shortcut cache, resources, Lumen versions, and
update staging are stored under the OS application-data directory. The host
does not search the working directory or silently apply environment defaults.

Lumen Hub is installed on demand from the official GitHub release pinned by
`lumen.lock.json` (currently v0.1.1). The Desktop binary contains an immutable
per-profile URL and SHA-256 catalog for `darwin-arm64-metal` and
`windows-x64-cpu` (plus the other official Desktop profiles), generated from
the Hub release manifest by `task lumen:sync` into
`internal/lumen/release_catalog.go`; it
never follows a mutable `latest` manifest. The archive is streamed into a
private staging directory, verified, probed, and promoted through
`lumen/current.json`. A hidden mode of the Desktop executable supervises the
actual Hub so abrupt parent death cannot leave an unowned process behind.

Hub configuration generation follows the upstream CLI/Launcher selection
pipeline (platform/release profile, backend, preset, region/cache, render, then
validate/write). Before installation, the Lumen panel offers the upstream
`minimal`, `basic`, and `brave` presets plus every pinned backend artifact for
the current platform, and a native directory picker for the model cache;
`basic`, the platform's conservative profile, and the private Desktop model
directory remain the defaults. The selected cache is canonicalized, rejected
when it is a filesystem root, and persisted with the preset before the release
installer runs. The result binds `127.0.0.1:50051`, advertises
only that loopback address through mDNS for compatibility with existing runtime
intents, stores downloaded models under the selected cache, and enables SigLIP,
InsightFace, PP-OCR, and BioCLIP. New Desktop
Server profiles include the loopback node explicitly while retaining mDNS
discovery for older intents and separately operated LAN nodes. To update the
pinned Hub release, update the catalog in `internal/lumen/release.go` from the
upstream `manifest.json`, mirror any control-plane proto change, regenerate its
Go bindings, and run the Desktop and Server gates.

The installed Hub's versioned Control service is also the source of truth for
the Lumen panel. Desktop subscribes to `WatchStatus` and presents the startup
phase, inference readiness, model download bytes/files, per-service phases,
backend, version, and structured failure details. It reads a bounded snapshot
of structured logs through `TailLogs` on demand and every five seconds while
the local Hub is connected. These protocol updates do not advance the Desktop
lifecycle version used to guard start/stop commands.

```sh
protoc -I internal/lumen/controlv1 -I /opt/homebrew/include \
  --go_out=paths=source_relative:internal/lumen/controlv1 \
  --go-grpc_out=paths=source_relative:internal/lumen/controlv1 \
  internal/lumen/controlv1/control.proto
```

The Settings window is a guided control surface, not a TOML prerequisite. On
first run, Go materialises a complete Desktop-local candidate and validates it
with the real strict Server loader. The user edits the projected library,
access, and runtime fields, sees an explicit draft/save state, and can inspect
the full TOML only from the optional Advanced tab. Reading or editing a draft
does not persist intent; only a successful Save or Apply changes the current
fingerprint.
