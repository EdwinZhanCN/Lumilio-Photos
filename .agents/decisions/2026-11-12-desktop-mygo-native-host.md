# Decision: the Desktop host is MyGo, and its Settings window is native UI

Status: implemented — owner decision 2026-11-12 (replace Wails outright, give
up the webview, rewrite the panel with native components; accepted costs: the
Go upgrade, a UI that is not a pixel copy, and a new update chain. No formal
release exists, so nothing has to stay compatible).

## Problem

The Desktop App is a tray application whose Settings window is opened rarely.
Under Wails v3 the window was a webview created hidden at launch, so its
WebKit helper processes lived for the whole run. Measured on macOS 27.2,
Apple silicon, both hosts configured, Server running, Settings not shown,
three paired samples: Wails 177/186/192 MB (host 113–125 plus helpers 64–67)
against 91/96/104 MB for the same Server under MyGo with no helper process.
About 88 MB of the steady state bought nothing.

The panel also carried a second toolchain (React, Vite+, generated Wails
bindings) for about 2,800 lines of logic, had no component test runner, and
the framework was itself pinned to an alpha with a hand-synchronized CLI.

## Decision

1. **The host is MyGo** (`github.com/egoist/mygo`). `desktop/internal/host` is
   the only package that imports its application API. The tray, the native
   menu, the single-instance lock, the folder picker, and the file manager
   are MyGo's.
2. **The Settings window is native UI** in `desktop/internal/panel`, written
   in Go and drawn by MyGo. It is created on demand and destroyed when
   closed. Its state lives in a `Panel` that outlives the window, so an
   unsaved draft survives closing it.
3. **The control plane is unchanged.** The window calls the same validating
   service facade in `desktop/internal/control`, and the runtime, storage,
   Lumen, shutdown, and preferences controllers were not modified. Only the
   RPC and event transport between them and the window disappeared.
4. **Views are tested without a window** with MyGo's tester. Control-plane
   calls run through injected `Go` and `Post` functions, inline in tests.
5. **Messages are ICU catalogs** in `desktop/internal/locales`, with typed
   generated accessors. A message nothing says fails a test, which keeps the
   extract-then-fill discipline the i18next extractor used to provide. A
   process status is localized by a stable `Key` on `ProcessPresentation`,
   not by matching its English label.
6. **Updates are MyGo's signed updater** behind `update.Provider`. The update
   controller keeps the aggregate (snapshot, operations, check → download →
   apply) and no longer downloads, verifies, or stages anything itself. A
   build has an update feed only when `desktop/build/update.pub` exists.
7. **Packaging is `mygo build`**, driven by `devtool package`. Native
   libraries are staged into `desktop/resources` before the link, relocatable,
   so the packager signs the bundle and writes the update archive in one
   step. Inno Setup and the hand-written bundle assembly are gone.
8. **The CLI is pinned by `go.mod`** (`tool` directive) and runs as
   `go tool mygo`. `mygo.config.ts` imports nothing, so the module has no
   JavaScript dependencies; Node only evaluates that one file.
9. **A system quit is a coordinated quit.** The app menu's Quit and SIGTERM
   go through the shutdown coordinator like the tray's Quit. Under Wails a
   SIGTERM killed the process.

## Alternatives considered

- **Stay on Wails and create the webview lazily.** It would have recovered
  most of the idle memory for a few lines. It lost because the owner wanted
  the webview and the second toolchain gone, not hidden, and because it
  would still leave the panel untested and the framework an alpha.
- **MyGo with a web frontend.** Keeps the React panel and swaps only the
  bindings. It lost for the same reason: it keeps the webview and its
  helpers whenever the window is open, and keeps the JavaScript toolchain.
- **Keep the hand-rolled update manifest and staging.** It had no transport:
  `ProviderAvailable` was always false. MyGo's updater brings the transport,
  signature verification, delta updates, and prereleases as a channel. The
  aggregate and its ordering stayed ours.
- **Patch the bundle after `mygo build`** (the previous dylibbundler step).
  It would invalidate the signature the packager just made and leave the
  update archive without the libraries. Staging before the link has neither
  problem and lets the build assert that the executable is self-contained.
- **A static `mygo.json`.** It cannot take the release version from the
  environment or turn updates on from the presence of a key file without a
  step that rewrites it. The computed config does both and needs no install.
- **A different native toolkit** (Fyne, Gio, or a Swift/WinUI split). MyGo was
  chosen by the owner; it is also the one that brings tray, updater,
  packaging, and catalogs together in Go, next to the in-process Server.

## Consequences

- MyGo is about seven weeks old and pre-1.0, and its `ui` API has already had
  one breaking migration. It is frozen in Renovate and bumped by hand with
  `go tool mygo migrate-ui` and the panel tests.
- The Desktop module requires Go 1.27.3. The Server module does not.
- Windows packaging changed shape and has not run; see the debt tracker.
- The bundle identifier `com.edwinzhan.lumilio-photos` is kept, because macOS
  keys folder permissions on it.
