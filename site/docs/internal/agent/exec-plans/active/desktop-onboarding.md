# Desktop onboarding & boot experience

Status: implemented (2026-07-06). Triggered by beta.1 Windows boot failures on a
tester's machine. W0–W3 landed; verified by `make desktop-test` and an arm64 DMG
build (`Lumilio-Photos-arm64.dmg`, v1.0.0-beta.3). Remaining: visual/UX pass of
the onboarding window on a real display and a native Windows smoke.

Scope: `desktop/` (Wails v3 host + supervisor) only. The in-browser first-run
wizard is **out of scope** and must not be duplicated — see the boundary below.

## Why

beta.1 on real Windows surfaced two classes of problem:

1. **Boot correctness/robustness** (bundled PostgreSQL):
   - `postgresql.conf` written with a raw Windows `log_directory` (backslashes);
     PostgreSQL's config parser treats `\` as an escape, so the postmaster
     FATALs at startup while `initdb` (which never reads the file) succeeds.
     **Fixed** — paths normalized to forward slashes (`pgConfPathValue`).
   - PG tool output captured as bytes rendered as mojibake on a zh-CN Windows
     (GBK). **Fixed** — subprocesses forced to `LC_ALL=C`/`LC_MESSAGES=C`.
   - `pg_ctl start` failure surfaced only "could not start server / Examine the
     log output". **Fixed** — the `postgres.log` tail is folded into the error.
   - Every spawned console tool (initdb/pg_ctl/pg_isready/createdb) popped a
     black console window (host is `-H windowsgui`, no console). **Fixed** —
     `CREATE_NO_WINDOW` via `hideConsole` (`proc_windows.go`/`proc_other.go`).
2. **Boot & install UX**: the tray shows a single `Starting…` string until
   `Supervisor.Start` returns; any slow/hung stage looks like a freeze with no
   cause, no progress, and no diagnosability.

## Desktop / web responsibility boundary (do not cross)

The in-browser `BootstrapWizard` (`web/src/features/auth/routes/BootstrapWizard.tsx`),
shown until the first admin exists, already owns and must remain the sole owner of:

- **Language & region** (in-browser UI language; `usePreference("language")`).
- **Admin account** (first registration becomes admin — server `SetupService`).
- **MFA** (TOTP / passkey / recovery codes).
- **Primary repository** name, storage strategy, duplicate handling.
- The **storage root**, which it renders **read-only**: *"Set by server
  configuration. The primary repository is created at `<root>/primary`."* The
  value comes from `repository_defaults.default_root`, which on desktop is fed
  by the supervisor via `DesktopParams.StoragePath`.

Data flow the desktop must respect:

```
desktop picks + validates storage root
  → server config storage_root (DesktopParams.StoragePath)
  → web wizard shows it read-only
  → creates <root>/primary
```

Therefore the desktop-native onboarding stays **thin** and only does what the
browser cannot or should not do for a packaged local app:

1. **ToS / open-source license acceptance** — a distribution-binary concern
   (bundles GPL exiftool/ffmpeg, PostgreSQL, pgvector). Docker/self-host users
   don't need this gate; the browser wizard has no such step. One-time, native.
2. **Storage root selection + writability validation** — the one storage
   responsibility that is desktop's (web treats root as read-only). Validate
   create/write/free-space *before* the server starts, then persist to
   `desktop-settings.json` so the wizard's read-only root reflects the choice.
3. **Failure diagnosability & staged progress** (see W1).

Anything else (accounts, MFA, repo strategy, in-browser language) is the
wizard's — the desktop native window must never re-implement it.

## Lumen / ML posture (deferred by design)

Keep ML **external-only and off the boot critical path**. Model download,
accelerator probing, config filling, and process diagnostics are exactly the
slow/fragile work that produced today's "starting" hang; they must never block
photo management.

- **Onboarding**: at most a one-line *hint* — "AI is optional, provided by a
  Lumen node (this machine or another on your LAN); enable later in Settings" —
  plus a skip. No config, no downloads in onboarding.
- **Future local hub control**: if the desktop later supervises a local
  lumen-hub (node discovery + process control), build it as a subsystem
  **parallel to and isolated from** the PostgreSQL lifecycle, opt-in from
  Settings, with its own cancellable/resumable progress + diagnostics. Its
  failure degrades to "AI unavailable", never "app won't start". Aligns with
  core-beliefs (ML optional; boring config; clean boot) and the existing
  `release-cicd.md` "ML external-only via mDNS" decision.

## Workstreams

### W0 — boot correctness & diagnosability — **landed / in-flight**
- ✅ conf path forward-slash normalization (`pgConfPathValue`, unit-tested).
- ✅ C-locale for PG subprocess messages (no mojibake).
- ✅ `postgres.log` tail folded into `pg_ctl start` errors.
- ✅ `CREATE_NO_WINDOW` for all spawned console tools.
- ✅ **Port-conflict pre-flight**: a foreign process on `localhost:6680` (a stale
  `go run ./cmd` dev server, a prior instance, a container) fooled `waitForServer`
  — the health probe accepts any status `< 500`, so a squatter's `404` reads as
  "ready" at ~T+0 while the real in-process server (binding `:6680` only after the
  ~10s Lumen mDNS timeout) then fails to bind and tears itself down (DB pool
  closed). The browser reaches the squatter → 404 on a fresh library. Fixed:
  `Supervisor.checkPortAvailable` binds `:6680` during *preparing* (before the
  expensive PG startup) and returns `ErrPortInUse`; `app.go` shows a localized
  "Port 6680 is already in use — quit that process and relaunch" dialog.
  Reproduced and verified both paths on macOS (busy → fail-fast, free → SPA 200).
- ✅ **Windows "starting" hang — root-caused & fixed.** Symptom: `postgres.log`
  shows the postmaster "ready to accept connections" yet the tray hangs at
  "starting database" forever, no `app.log`, both postgres and the app process
  alive. Cause: `pg_ctl start` spawns the long-lived postmaster as a grandchild
  that inherits `pg_ctl`'s stdout/stderr; when those are an `os/exec` capture
  pipe (`output()` → `bytes.Buffer`), the postmaster holds the pipe's write end
  open for its whole life, so the stdout-copier never sees EOF and `cmd.Wait()`
  (thus `Postgres.Start`) blocks indefinitely even though PG is up — `app.Run`
  is never reached. (Unix is immune: `pg_ctl` `setsid`s and redirects the
  postmaster's stdio to the logfile.) Fix: `Postgres.Start` now uses
  `runToFile` — pg_ctl's output goes to a real `pg_ctl.log` file, so Go passes
  the handle directly with no pipe/copier and `Run` returns when pg_ctl exits.
  Verified by the full PG lifecycle smoke on macOS; needs a fresh Windows build
  to confirm on the tester's VM.

### W1 — staged startup with progress, per-stage timeout & failure surfacing — **landed**
Split `Supervisor.Start` (`desktop/supervisor/supervisor.go`) into named,
individually-bounded stages, each reporting to a status callback so the tray
(`desktop/app.go`) shows `Initializing database…` / `Starting database…` /
`Starting server…` / `Ready`, and on failure shows a human-readable reason +
the log directory path — instead of one static `Starting…` that can hang ~2.5m.
- ✅ `Options.OnStage func(stage string)` reports stage keys
  (`Stage{Preparing,InitDB,StartingDB,StartingServer,Ready}`); `app.go`'s
  `onStage` localizes them and re-renders the tray via `refreshMenu()`.
- ✅ Per-stage timeouts kept (pg_ctl `-w -t`, `WaitReady`, server health); the
  most recent stage is tracked in `app.go` (`lastStage`).
- ✅ The failure dialog (`failureMessage`) names the failing stage, shows the
  cause, and points at `Supervisor.LogDir()`.
- Note: migrations run inside `server/app.Run`; they are reported under
  `Starting server…` (the honest granularity available without server hooks).

### W2 — thin native onboarding window — **landed**
A small Wails webview window shown on first run (gated on
`desktop-settings.json`'s `onboarding_completed`), covering exactly the three
desktop-only concerns:
1. ✅ **ToS / OSS license** acceptance (persisted as `tos_accepted_version`;
   bump `tosVersion` in `onboarding.go` to re-prompt).
2. ✅ **Storage root** picker (native `Dialog.OpenFile` directory chooser) + live
   writability/free-space validation (`validateStorage` probes the nearest
   existing ancestor, never creating the dir prematurely); persisted via
   `Supervisor.SaveSettings` before the server starts, so the browser wizard's
   read-only root is correct.
3. ✅ Native-chrome **i18n** (`desktop/strings.go`): a small zh/en table for
   tray/dialog/stage surfaces only, plus the window's own JS i18n; default from
   OS locale (`detectOSLang`), overridden by the in-window language toggle.

Implementation:
- The window is the app's **only** webview. There is no Wails binding
  generation: the setup page (`desktop/onboarding/index.html`, self-contained)
  talks to Go through a plain-`fetch` JSON API served by the Wails asset handler
  (`desktopApp.onboardingHandler`): `GET /__onb/state`, `POST /__onb/pick`,
  `POST /__onb/complete`.
- Sequencing (`app.go` `boot()`): `NeedsOnboarding` → show window → block on
  `/__onb/complete` → close window → `Supervisor.Start` (staged, W1) →
  auto-open browser → in-browser `BootstrapWizard`. Closing the window before
  completion quits (no half-configured boot).
- Cross-platform disk-free lives in `disk_unix.go` / `disk_windows.go`.
- Did **not** reintroduce account/MFA/repo-strategy steps (web wizard's).

**Design constraints** (this native window; distinct from the web app's daisyUI):
- **shadcn/ui, minimalist.** Restrained, content-first layout; no chrome for its
  own sake. This window is small and single-purpose.
- **lucide** for iconography.
- **Apple HIG conformance (at minimum).** Native spacing/type rhythm, clear
  primary action, generous margins, standard control affordances.
- **Preserve window safe-area.** Respect the OS-reserved insets — the Wails
  title-bar/traffic-light region on macOS and the drag region on a
  frameless/translucent window; content must not underlap them. Use CSS
  `env(safe-area-inset-*)` and a defined drag region rather than fixed offsets.
- **Offline-first caveat (must resolve, do not ignore):** first run may have no
  network, and a local-first app must render its own setup UI offline. A live
  runtime CDN fetch for lucide/shadcn assets would break that and clash with the
  bundle's CSP. Resolution: **vendor** the lucide distributable (and any
  shadcn/ui build output) into `desktop/assets/` so the window is fully
  self-contained; "CDN" here means the upstream source we pin/copy from, not a
  runtime dependency. If shadcn/ui (React + build) is too heavy for one small
  window, port its visual language onto the existing vanilla-JS onboarding
  assets rather than pulling in a build toolchain.
- **Resolution taken:** a single self-contained `onboarding/index.html` — no
  React, no build step, no node deps. It carries shadcn/ui-derived neutral
  design tokens (light+dark via `prefers-color-scheme`), lucide icons inlined as
  SVG (no runtime CDN), a top drag strip + `env(safe-area-inset-*)` padding so
  content never underlaps the traffic lights, and an EN/中文 segmented toggle.
  Framework permission was on the table but a build toolchain for one small
  offline window was not worth its weight; the vanilla page matches the look.

### W3 — Lumen/ML hint (thin) + defer local hub control — **landed**
- ✅ Optional one-line ML hint on the onboarding window (`ai.note`, zh/en): AI is
  optional, provided by a Lumen node on this machine or the LAN, enable later in
  Settings, nothing downloaded now. No config, no downloads, no action.
- Local lumen-hub control remains a future, isolated, opt-in subsystem (see the
  Lumen/ML posture section and `release-cicd.md`); no implementation this cycle.

## Verification
- ✅ `make desktop-test` — supervisor unit tests + new main-package tests
  (`onboarding_test.go`: `validateStorage`, `humanBytes`, `normalizeLang`, and
  the `/__onb/state|complete` handlers incl. reject-on-decline / reject-on-
  unwritable). PG smoke auto-skips without bundled binaries.
- ✅ arm64 `.app` + DMG built (`desktop/scripts/build-macos.sh arm64 --dmg`,
  v1.0.0-beta.3); onboarding assets confirmed embedded, version stamped.
- ✅ Smoke-launched the built app with a throwaway `LUMILIO_APP_DATA`: boots the
  onboarding path, Wails asset handler wired (`handler=true`), no errors.
  (Automated screenshot blocked by the shell's macOS TCC permissions — visual
  UX pass on a real display is the remaining manual step.)
- Manual smoke on Windows: a Parallels **Windows 11** VM is sufficient for UX
  iteration (cmd windows, SmartScreen "More info → Run anyway", storage picker,
  staged progress, hang localization). Caveat: on Apple Silicon it runs
  windows/amd64 under x64 emulation — **functional** validation only, not a
  performance signal. Do a final pass on a **native amd64** clean machine (no
  VC++/dev tools) before release.
- Code signing (Authenticode; EV to skip SmartScreen reputation) remains a
  pre-release upgrade over the portable zip — tracked in `release-cicd.md` W4.

## Critical files
- `desktop/supervisor/supervisor.go` — staged `Start` + `OnStage` callback,
  `Settings`/`SaveSettings`/`NeedsOnboarding`/`LogDir`/`DefaultStoragePath` (W1/W2).
- `desktop/app.go` — tray status/progress rendering, onboarding gating in
  `boot()`, failure dialog with stage + log dir (W1/W2).
- `desktop/onboarding.go` — asset handler + JSON API, window creation,
  `validateStorage`; `desktop/onboarding/index.html` — the self-contained UI.
- `desktop/strings.go` — native-chrome zh/en tables (W2 i18n).
- `desktop/disk_unix.go` / `desktop/disk_windows.go` — free-space per platform.
- `desktop/supervisor/config.go` / `desktop-settings.json` — storage path +
  `onboarding_completed` / `tos_accepted_version` / `language` persistence (W2).
- `desktop/supervisor/postgres.go` — boot correctness fixes landed (W0).
- `desktop/scripts/build-macos.sh` — stamps `main.buildVersion` (W2).
- `web/src/features/auth/routes/BootstrapWizard.tsx` — the boundary reference;
  read-only storage root the desktop must feed (do not modify).
