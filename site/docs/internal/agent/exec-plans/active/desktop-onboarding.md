# Desktop onboarding & boot experience

Status: planning (2026-07-05). Triggered by beta.1 Windows boot failures on a
tester's machine.

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
- ⏳ **Root-cause the "starting" hang**: collect from the tester —
  `%LocalAppData%\Lumilio Photos\postgres\17\logs\postgres.log` (postmaster
  actually ready?) and `%LocalAppData%\Lumilio Photos\logs\` (in-process server
  log — prime suspect: migrations / `CREATE EXTENSION vector`, since pgvector is
  nmake-built separately). The black window that *persists* is likely the
  postmaster itself, i.e. PG is up and a later stage is stuck.

### W1 — staged startup with progress, per-stage timeout & failure surfacing
Split `Supervisor.Start` (`desktop/supervisor/supervisor.go`) into named,
individually-bounded stages, each reporting to a status callback so the tray
(`desktop/app.go`) shows `Initializing database…` / `Starting database…` /
`Running migrations…` / `Ready`, and on failure shows a human-readable reason +
the log directory path — instead of one static `Starting…` that can hang ~2.5m.
- Add a progress/status callback to `Options` (or a channel); `app.go`
  `refreshMenu()` renders the current stage.
- Keep per-stage timeouts (pg_ctl `-w -t`, `WaitReady`, server health) but make
  the failing stage nameable in the error and in the tray.
- Error dialogs (`app.go:97`) already show `err.Error()`; ensure each stage
  error is actionable (cause + `%LocalAppData%\Lumilio Photos\logs`).

### W2 — thin native onboarding window
A small Wails window shown on first run (gated on a `desktop-settings.json`
flag), covering exactly the three desktop-only concerns:
1. **ToS / OSS license** acceptance (persist accepted version).
2. **Storage root** picker + live writability/free-space validation; persist via
   `Supervisor.SetStoragePath` → `desktop-settings.json`; only then start the
   server so the browser wizard's read-only root is correct.
3. Native-chrome **i18n** (tray menu, dialogs, this window) — a small zh/en
   string table for desktop-native surfaces only (independent of the in-browser
   language the wizard owns); default from OS locale.
- Sequencing: onboarding window → validated storage path → `Supervisor.Start`
  (staged, W1) → auto-open browser → in-browser `BootstrapWizard`.
- Reuse the discarded onboarding commit's assets only where they fit this
  narrowed scope; do **not** reintroduce account/MFA/repo-strategy steps.

### W3 — Lumen/ML hint (thin) + defer local hub control
- Add the optional one-line ML hint + skip to the onboarding window (W2). No
  config or downloads.
- Record the "local lumen-hub control is a future, isolated, opt-in subsystem"
  decision here and in `release-cicd.md`; no implementation this cycle.

## Verification
- `make desktop-test` (supervisor unit tests; PG smoke auto-skips without
  bundled binaries).
- Manual smoke on Windows: a Parallels **Windows 11** VM is sufficient for UX
  iteration (cmd windows, SmartScreen "More info → Run anyway", storage picker,
  staged progress, hang localization). Caveat: on Apple Silicon it runs
  windows/amd64 under x64 emulation — **functional** validation only, not a
  performance signal. Do a final pass on a **native amd64** clean machine (no
  VC++/dev tools) before release.
- Code signing (Authenticode; EV to skip SmartScreen reputation) remains a
  pre-release upgrade over the portable zip — tracked in `release-cicd.md` W4.

## Critical files
- `desktop/supervisor/supervisor.go` — staged `Start`, status callback (W1).
- `desktop/app.go` — tray status/progress rendering + failure dialogs (W1).
- `desktop/supervisor/postgres.go` — boot correctness fixes landed (W0).
- `desktop/supervisor/config.go` / `desktop-settings.json` — storage path +
  onboarding/ToS flags persistence (W2).
- `web/src/features/auth/routes/BootstrapWizard.tsx` — the boundary reference;
  read-only storage root the desktop must feed (do not modify).
