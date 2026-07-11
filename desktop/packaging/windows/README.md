# Windows installer (Inno Setup)

`lumilio.iss` packages the portable app directory from
`desktop/scripts/build-windows.sh` into a per-user `setup.exe`.

## What it does

- **Per-user install** to `%LocalAppData%\Programs\Lumilio Photos` — no
  administrator rights, no UAC prompt (the VS Code / Discord model). The app also
  keeps its runtime data under `%LocalAppData%`, so install scope and data scope
  match.
- **WebView2 Runtime** — the first-run onboarding window is a WebView2 surface.
  The installer checks the EdgeUpdate registry keys and, if the runtime is
  missing, downloads Microsoft's Evergreen bootstrapper and installs it silently.
  If there is no network it warns and lets the user continue (they can install it
  later). The runtime is a shared system component and is **never** removed on
  uninstall.
- **Shortcuts** — Start Menu always, Desktop optional (unchecked task).
- **Uninstaller** (auto-registered in "Apps & features") —
  - stops the running app and its bundled PostgreSQL first (force-kills the
    process tree; PostgreSQL is WAL-crash-safe and the supervisor clears a stale
    `postmaster.pid` on next launch),
  - removes the program files,
  - offers to also delete the app data in `%LocalAppData%\Lumilio Photos`
    (default **No**),
  - if the photo library lives inside the data dir (the default location),
    requires a second explicit confirmation before deleting, because that is the
    user's original photos. An **external** library (e.g. `D:\Photos`) is never
    touched — the uninstaller only ever deletes the data dir.
- **In-place upgrades** — the same `setup.exe` for a newer version reuses the
  stable `AppId` and install dir. Before copying the new payload it deletes
  everything under `{app}` (`[InstallDelete]`), so removed DLLs/tools from older
  builds do not linger. App data and the photo library are never touched.

Storage-path selection is deliberately **not** in the installer: it belongs to
the app's first-run onboarding window (per-user, with live writability
validation), which renders correctly once WebView2 is present.

## Build

On Windows with [Inno Setup](https://jrsoftware.org/isdl.php) 6.1+ (for the
`DownloadTemporaryFile` support):

```bat
:: after desktop\scripts\build-windows.sh has produced build\windows\Lumilio Photos\
ISCC.exe /DAppVersion=1.2.3 desktop\packaging\windows\lumilio.iss
```

Output: `desktop\build\Lumilio-Photos-1.2.3-windows-amd64-setup.exe`.

Override the payload location with `/DPayloadDir=<path>` if it is not the default
`..\..\build\windows\Lumilio Photos` (relative to the `.iss`).

## Not signed

The setup.exe is unsigned, so SmartScreen shows "Windows protected your PC →
More info → Run anyway" on first run — the same posture as the unsigned macOS
DMG. An Authenticode (ideally EV) certificate removes it; tracked with the
broader signing work in `release-cicd.md`.
