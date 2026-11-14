# Lumilio Photos Desktop

This is the MyGo desktop host for Lumilio Photos. Server and SQLite run
in-process; the optional Lumen Hub is a separately supervised child process.
The tray and the Settings window render the same typed control-plane snapshot.

The Settings window is native UI, written in Go and drawn by MyGo: there is no
webview, HTML, or JavaScript. It is created on demand and destroyed when
closed, so the tray-only app keeps nothing in memory for it.

MyGo and its CLI are pinned together by `go.mod` (`require` and `tool`). The
CLI always runs as `go tool mygo`; there is nothing to install separately.
`mygo.config.ts` imports nothing and is evaluated by Node or Bun, so the
module has no JavaScript dependencies.

Useful local commands, from the repository root:

```sh
task desktop:dev            # run the app, rebuilding as sources change
task desktop:test           # Go tests with the race detector
task desktop:panel:check    # catalogs, generated accessors, view vet
task desktop:resources      # fetch the pinned media tools
task desktop:package        # stage resources and package with mygo build
```

`go build` and `go test` need neither the CLI nor Node.

## Settings window

`internal/panel` holds the window's state and builds its interface for every
frame; `internal/host` owns the tray, the window's lifetime, and the
coordinated quit. Views are tested without a window with `ui.NewTester`
(`internal/panel/panel_test.go`): prefer a failing view test to a manual
check.

Every string a person reads lives in `internal/locales`, one ICU MessageFormat
catalog per language. `catalog_gen.go` is generated from them and committed:

```sh
cd desktop
go tool mygo generate       # after changing a catalog
go tool mygo i18n check
```

Add a message to `en.json` first, then to every other catalog. A message
nothing says fails `go test ./internal/locales/`.

## Packaging

`go run ./cmd/devtool package` stages what the app ships beside its executable
into `resources/` (the Web SPA, license notices, and the native libraries of
the current platform), then runs `mygo build`, which compiles, bundles, signs,
and, with `--installer`, makes the disk image or setup program.

Native libraries are staged before the app is linked, not patched in
afterwards, because the packager signs the bundle and writes the update
archive in one step. On macOS the libvips dylib closure and its modules are
made relocatable (`@rpath` names, one rpath on the executable) and the build
then checks that the packaged executable refers to nothing outside the
bundle. On Windows the mingw64 DLL closure is staged beside the executable,
and packaging must run in MSYS2 MINGW64.

The release identity comes from the environment: `LUMILIO_VERSION` is the
product version the app reports and the updater compares.

## Updates

Updates use MyGo's signed updater: `mygo build` signs each platform's update
archive, and the app verifies it against the public key built into it before
replacing itself. The Stable channel reads the latest full GitHub release and
Beta also reads prereleases.

A build has an update feed only when `build/update.pub` exists. To turn
updates on, create the key pair once and commit the public half:

```sh
cd desktop
go tool mygo keygen -o /path/outside/the/repository
cp /path/outside/the/repository/mygo-update.pub build/update.pub
```

Store the secret half as the `MYGO_UPDATER_PRIVATE_KEY` secret of the release
workflow and nowhere in the repository: installed apps accept only updates
signed with it, so losing it strands them, and anyone who has it can ship code
to them. Without the file the app says in Settings that no signed update
provider is configured.

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
access, and runtime fields and sees an explicit draft/save state; the complete
manifest stays a file that Settings can reveal. Reading or editing a draft
does not persist intent; only a successful Save changes the current
fingerprint.

`internal/storage` remains the native grant and tray-shortcut host. Shortcuts
open a Storage Location folder from a readable registered path; catalog Location
status does not gate `CanOpen` or reveal. Native authorization nonces and
approval paths stay in the Desktop control plane while Web Storage admin drives
the durable `/api/v1/storage/native-tasks` lifecycle.
