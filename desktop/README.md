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

The Settings window is a guided control surface, not a TOML prerequisite. On
first run, Go materialises a complete Desktop-local candidate and validates it
with the real strict Server loader. The user edits the projected library,
access, and runtime fields, sees an explicit draft/save state, and can inspect
the full TOML only from the optional Advanced tab. Reading or editing a draft
does not persist intent; only a successful Save or Apply changes the current
fingerprint.
