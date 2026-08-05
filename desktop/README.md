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

Lumen Hub is installed on demand from the immutable GitHub release pinned by
`../lumen.lock.json`. The Desktop binary embeds the generated artifact URL and
SHA-256 catalog in `internal/lumen/release_catalog.go`; it never follows a
mutable `latest` endpoint. Downloads are streamed into a private staging
directory, verified, probed, and promoted through `lumen/current.json`. A
hidden Desktop mode owns the Hub process tree so abrupt parent exit cannot
leave an unmanaged child.

Desktop owns only managed-Hub intent: release profile, preset, download region,
and model-cache directory. Immediately before every start it resolves the
exact installed Hub binary and invokes `lumen-hub config render --target
desktop`. The Hub's own `lumen-schema` implementation validates and renders the
complete configuration, which Desktop then promotes atomically. Desktop does
not duplicate model IDs, service graphs, backend runtime strings, batching
defaults, or YAML validation. Re-rendering on each start also prevents an old
derived file from silently becoming a second configuration authority.

The available preset IDs and platform artifacts are generated from the pinned
release by `task lumen:sync`. To upgrade, publish Hub first, then run
`task lumen:sync RELEASE=<tag>` at the Photos repository root and commit the
lock, generated catalog, and any vendored control-proto change together.

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
